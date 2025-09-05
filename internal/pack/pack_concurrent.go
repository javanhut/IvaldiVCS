package pack

import (
	"bytes"
	"compress/zlib"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"runtime"
	"sync"

	"github.com/klauspost/compress/zstd"
)

// Default number of workers for concurrent compression
const DefaultWorkers = 8

// CompressedObject represents a compressed object ready for packing
type CompressedObject struct {
	Index      int    // Original index in input array
	Header     []byte // Object header (type + size)
	Compressed []byte // Compressed data
	Error      error  // Any compression error
}

// CompressionJob represents a job for the worker pool
type CompressionJob struct {
	Index  int
	Object Object
	Result chan<- CompressedObject
}

// CompressionPool manages concurrent compression workers
type CompressionPool struct {
	workers  int
	jobs     chan CompressionJob
	wg       sync.WaitGroup
	zlibPool sync.Pool
	zstdPool sync.Pool
}

// NewCompressionPool creates a new compression worker pool
func NewCompressionPool(workers int) *CompressionPool {
	if workers <= 0 {
		workers = runtime.NumCPU()
		if workers > DefaultWorkers {
			workers = DefaultWorkers
		}
	}

	pool := &CompressionPool{
		workers: workers,
		jobs:    make(chan CompressionJob, workers*2),
		zlibPool: sync.Pool{
			New: func() interface{} {
				return &bytes.Buffer{}
			},
		},
		zstdPool: sync.Pool{
			New: func() interface{} {
				enc, _ := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedDefault))
				return enc
			},
		},
	}

	// Start workers
	for i := 0; i < workers; i++ {
		pool.wg.Add(1)
		go pool.worker()
	}

	return pool
}

// worker processes compression jobs
func (p *CompressionPool) worker() {
	defer p.wg.Done()

	for job := range p.jobs {
		result := p.compressObject(job.Index, job.Object)
		job.Result <- result
	}
}

// compressObject compresses a single object
func (p *CompressionPool) compressObject(index int, obj Object) CompressedObject {
	// Generate object header
	var headerBuf bytes.Buffer
	if err := writeObjHeader(&headerBuf, obj.Type, obj.Size); err != nil {
		return CompressedObject{
			Index: index,
			Error: fmt.Errorf("write header: %w", err),
		}
	}

	// Get buffer from pool
	buf := p.zlibPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer p.zlibPool.Put(buf)

	// Compress based on algorithm
	var err error
	switch obj.Algo {
	case CompressZlib:
		err = p.compressZlibPooled(buf, obj.Data)
	case CompressZstd:
		err = p.compressZstdPooled(buf, obj.Data)
	default:
		err = fmt.Errorf("unknown compression algo: %v", obj.Algo)
	}

	if err != nil {
		return CompressedObject{
			Index: index,
			Error: err,
		}
	}

	// Create result with copied data
	compressed := make([]byte, buf.Len())
	copy(compressed, buf.Bytes())

	return CompressedObject{
		Index:      index,
		Header:     headerBuf.Bytes(),
		Compressed: compressed,
		Error:      nil,
	}
}

// compressZlibPooled uses pooled resources for zlib compression
func (p *CompressionPool) compressZlibPooled(dst *bytes.Buffer, data []byte) error {
	zw := zlib.NewWriter(dst)
	if _, err := zw.Write(data); err != nil {
		return err
	}
	return zw.Close()
}

// compressZstdPooled uses pooled encoder for zstd compression
func (p *CompressionPool) compressZstdPooled(dst *bytes.Buffer, data []byte) error {
	enc := p.zstdPool.Get().(*zstd.Encoder)
	defer p.zstdPool.Put(enc)

	enc.Reset(dst)
	if _, err := enc.Write(data); err != nil {
		return err
	}
	return enc.Close()
}

// Submit submits objects for compression
func (p *CompressionPool) Submit(objects []Object) ([]CompressedObject, error) {
	resultChan := make(chan CompressedObject, len(objects))

	// Submit all jobs
	for i, obj := range objects {
		p.jobs <- CompressionJob{
			Index:  i,
			Object: obj,
			Result: resultChan,
		}
	}

	// Collect results
	results := make([]CompressedObject, len(objects))
	for i := 0; i < len(objects); i++ {
		result := <-resultChan
		if result.Error != nil {
			return nil, fmt.Errorf("compress object %d: %w", result.Index, result.Error)
		}
		results[result.Index] = result
	}

	return results, nil
}

// Close shuts down the worker pool
func (p *CompressionPool) Close() {
	close(p.jobs)
	p.wg.Wait()
}

// WritePackConcurrent writes a pack file using concurrent compression
func WritePackConcurrent(objs []Object, trailerSHA256 bool, workers int) ([]byte, error) {
	var body bytes.Buffer

	// Write header
	if _, err := body.Write(magicPACK); err != nil {
		return nil, err
	}
	if err := binary.Write(&body, binary.BigEndian, packVersion); err != nil {
		return nil, err
	}
	count := uint32(len(objs))
	if err := binary.Write(&body, binary.BigEndian, count); err != nil {
		return nil, err
	}

	// Create compression pool
	pool := NewCompressionPool(workers)
	defer pool.Close()

	// Compress objects concurrently
	compressed, err := pool.Submit(objs)
	if err != nil {
		return nil, fmt.Errorf("concurrent compression failed: %w", err)
	}

	// Write compressed objects in order
	for _, comp := range compressed {
		if _, err := body.Write(comp.Header); err != nil {
			return nil, err
		}
		if _, err := body.Write(comp.Compressed); err != nil {
			return nil, err
		}
	}

	// Optional SHA-256 trailer
	if trailerSHA256 {
		sum := sha256.Sum256(body.Bytes())
		if _, err := body.Write(sum[:]); err != nil {
			return nil, err
		}
	}

	return body.Bytes(), nil
}

// BatchCompressor provides a high-level interface for concurrent compression
type BatchCompressor struct {
	pool    *CompressionPool
	mu      sync.Mutex
	batches map[string][]CompressedObject
}

// NewBatchCompressor creates a new batch compressor
func NewBatchCompressor(workers int) *BatchCompressor {
	return &BatchCompressor{
		pool:    NewCompressionPool(workers),
		batches: make(map[string][]CompressedObject),
	}
}

// CompressBatch compresses a batch of objects and stores results
func (bc *BatchCompressor) CompressBatch(batchID string, objects []Object) error {
	compressed, err := bc.pool.Submit(objects)
	if err != nil {
		return err
	}

	bc.mu.Lock()
	bc.batches[batchID] = compressed
	bc.mu.Unlock()

	return nil
}

// GetBatch retrieves compressed objects for a batch
func (bc *BatchCompressor) GetBatch(batchID string) ([]CompressedObject, bool) {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	batch, exists := bc.batches[batchID]
	return batch, exists
}

// WritePack writes a pack file from a batch
func (bc *BatchCompressor) WritePack(batchID string, trailerSHA256 bool) ([]byte, error) {
	bc.mu.Lock()
	compressed, exists := bc.batches[batchID]
	bc.mu.Unlock()

	if !exists {
		return nil, fmt.Errorf("batch %s not found", batchID)
	}

	var body bytes.Buffer

	// Write header
	if _, err := body.Write(magicPACK); err != nil {
		return nil, err
	}
	if err := binary.Write(&body, binary.BigEndian, packVersion); err != nil {
		return nil, err
	}
	count := uint32(len(compressed))
	if err := binary.Write(&body, binary.BigEndian, count); err != nil {
		return nil, err
	}

	// Write compressed objects
	for _, comp := range compressed {
		if _, err := body.Write(comp.Header); err != nil {
			return nil, err
		}
		if _, err := body.Write(comp.Compressed); err != nil {
			return nil, err
		}
	}

	// Optional SHA-256 trailer
	if trailerSHA256 {
		sum := sha256.Sum256(body.Bytes())
		if _, err := body.Write(sum[:]); err != nil {
			return nil, err
		}
	}

	return body.Bytes(), nil
}

// Close shuts down the batch compressor
func (bc *BatchCompressor) Close() {
	bc.pool.Close()
}
