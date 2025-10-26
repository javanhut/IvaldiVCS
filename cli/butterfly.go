package cli

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/javanhut/Ivaldi-vcs/internal/butterfly"
	"github.com/javanhut/Ivaldi-vcs/internal/cas"
	"github.com/javanhut/Ivaldi-vcs/internal/history"
	"github.com/javanhut/Ivaldi-vcs/internal/refs"
	"github.com/spf13/cobra"
)

var butterflyCmd = &cobra.Command{
	Use:     "butterfly <name>",
	Aliases: []string{"bf"},
	Short:   "Create or manage butterfly timelines",
	Long:    `Butterfly timelines are experimental sandboxes that branch from a parent timeline`,
	Args:    cobra.MinimumNArgs(1),
	RunE:    butterflyCreateRun,
}

var butterflyUpCmd = &cobra.Command{
	Use:   "up",
	Short: "Sync butterfly up to parent (merge to parent)",
	Args:  cobra.NoArgs,
	RunE:  butterflySyncUpRun,
}

var butterflyDownCmd = &cobra.Command{
	Use:   "down",
	Short: "Sync parent down to butterfly (merge from parent)",
	Args:  cobra.NoArgs,
	RunE:  butterflySyncDownRun,
}

var butterflyRemoveCmd = &cobra.Command{
	Use:     "rm <name>",
	Aliases: []string{"remove"},
	Short:   "Remove a butterfly timeline",
	Args:    cobra.ExactArgs(1),
	RunE:    butterflyRemoveRun,
}

var cascadeDelete bool

func init() {
	butterflyRemoveCmd.Flags().BoolVar(&cascadeDelete, "cascade", false, "Delete nested butterflies recursively")
}

func butterflyCreateRun(cmd *cobra.Command, args []string) error {
	name := args[0]

	if name == "up" || name == "down" || name == "rm" || name == "remove" {
		return fmt.Errorf("'%s' is a butterfly subcommand, not a valid butterfly name", name)
	}

	ivaldiDir := ".ivaldi"
	if _, err := os.Stat(ivaldiDir); os.IsNotExist(err) {
		return fmt.Errorf("not in an Ivaldi repository (no .ivaldi directory found)")
	}

	refsManager, err := refs.NewRefsManager(ivaldiDir)
	if err != nil {
		return fmt.Errorf("failed to initialize refs manager: %w", err)
	}
	defer refsManager.Close()

	currentTimeline, err := refsManager.GetCurrentTimeline()
	if err != nil {
		return fmt.Errorf("no current timeline found: %w", err)
	}

	currentTimelineRef, err := refsManager.GetTimeline(currentTimeline, refs.LocalTimeline)
	if err != nil {
		return fmt.Errorf("failed to get current timeline: %w", err)
	}

	objectsDir := filepath.Join(ivaldiDir, "objects")
	casStore, err := cas.NewFileCAS(objectsDir)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}

	mmr, err := history.NewPersistentMMR(casStore, ivaldiDir)
	if err != nil {
		return fmt.Errorf("failed to initialize MMR: %w", err)
	}
	defer mmr.Close()

	bfManager, err := butterfly.NewManager(ivaldiDir, casStore, refsManager, mmr)
	if err != nil {
		return fmt.Errorf("failed to initialize butterfly manager: %w", err)
	}
	defer bfManager.Close()

	var divergenceHash cas.Hash
	copy(divergenceHash[:], currentTimelineRef.Blake3Hash[:])

	err = bfManager.CreateButterfly(name, currentTimeline, divergenceHash)
	if err != nil {
		return fmt.Errorf("failed to create butterfly: %w", err)
	}

	fmt.Printf("Creating butterfly timeline '%s' from '%s'\n", name, currentTimeline)
	fmt.Printf("Divergence point: %s\n", hex.EncodeToString(divergenceHash[:])[:16])
	fmt.Printf("✓ Created butterfly '%s'\n", name)

	err = refsManager.SetCurrentTimeline(name)
	if err != nil {
		return fmt.Errorf("failed to switch to butterfly: %w", err)
	}

	fmt.Printf("✓ Switched to butterfly timeline\n")

	return nil
}

func butterflySyncUpRun(cmd *cobra.Command, args []string) error {
	ivaldiDir := ".ivaldi"
	if _, err := os.Stat(ivaldiDir); os.IsNotExist(err) {
		return fmt.Errorf("not in an Ivaldi repository (no .ivaldi directory found)")
	}

	refsManager, err := refs.NewRefsManager(ivaldiDir)
	if err != nil {
		return fmt.Errorf("failed to initialize refs manager: %w", err)
	}
	defer refsManager.Close()

	currentTimeline, err := refsManager.GetCurrentTimeline()
	if err != nil {
		return fmt.Errorf("no current timeline found: %w", err)
	}

	objectsDir := filepath.Join(ivaldiDir, "objects")
	casStore, err := cas.NewFileCAS(objectsDir)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}

	mmr, err := history.NewPersistentMMR(casStore, ivaldiDir)
	if err != nil {
		return fmt.Errorf("failed to initialize MMR: %w", err)
	}
	defer mmr.Close()

	bfManager, err := butterfly.NewManager(ivaldiDir, casStore, refsManager, mmr)
	if err != nil {
		return fmt.Errorf("failed to initialize butterfly manager: %w", err)
	}
	defer bfManager.Close()

	if !bfManager.IsButterfly(currentTimeline) {
		return fmt.Errorf("'%s' is not a butterfly timeline", currentTimeline)
	}

	bf, err := bfManager.GetButterflyInfo(currentTimeline)
	if err != nil {
		return err
	}

	syncer := butterfly.NewSyncer(bfManager, casStore, refsManager, mmr)

	fmt.Printf("Syncing butterfly '%s' up to parent '%s'...\n", currentTimeline, bf.ParentName)

	err = syncer.SyncUp(currentTimeline)
	if err != nil {
		return fmt.Errorf("failed to sync up: %w", err)
	}

	parentRef, _ := refsManager.GetTimeline(bf.ParentName, refs.LocalTimeline)
	fmt.Printf("✓ Parent '%s' now at: %s\n", bf.ParentName, hex.EncodeToString(parentRef.Blake3Hash[:])[:16])
	fmt.Printf("✓ Butterfly synchronized\n")

	return nil
}

func butterflySyncDownRun(cmd *cobra.Command, args []string) error {
	ivaldiDir := ".ivaldi"
	if _, err := os.Stat(ivaldiDir); os.IsNotExist(err) {
		return fmt.Errorf("not in an Ivaldi repository (no .ivaldi directory found)")
	}

	refsManager, err := refs.NewRefsManager(ivaldiDir)
	if err != nil {
		return fmt.Errorf("failed to initialize refs manager: %w", err)
	}
	defer refsManager.Close()

	currentTimeline, err := refsManager.GetCurrentTimeline()
	if err != nil {
		return fmt.Errorf("no current timeline found: %w", err)
	}

	objectsDir := filepath.Join(ivaldiDir, "objects")
	casStore, err := cas.NewFileCAS(objectsDir)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}

	mmr, err := history.NewPersistentMMR(casStore, ivaldiDir)
	if err != nil {
		return fmt.Errorf("failed to initialize MMR: %w", err)
	}
	defer mmr.Close()

	bfManager, err := butterfly.NewManager(ivaldiDir, casStore, refsManager, mmr)
	if err != nil {
		return fmt.Errorf("failed to initialize butterfly manager: %w", err)
	}
	defer bfManager.Close()

	if !bfManager.IsButterfly(currentTimeline) {
		return fmt.Errorf("'%s' is not a butterfly timeline", currentTimeline)
	}

	bf, err := bfManager.GetButterflyInfo(currentTimeline)
	if err != nil {
		return err
	}

	syncer := butterfly.NewSyncer(bfManager, casStore, refsManager, mmr)

	fmt.Printf("Syncing butterfly '%s' down from parent '%s'...\n", currentTimeline, bf.ParentName)

	err = syncer.SyncDown(currentTimeline)
	if err != nil {
		return fmt.Errorf("failed to sync down: %w", err)
	}

	fmt.Printf("✓ Merged successfully\n")
	fmt.Printf("✓ Butterfly now includes parent's latest changes\n")

	return nil
}

func butterflyRemoveRun(cmd *cobra.Command, args []string) error {
	name := args[0]

	ivaldiDir := ".ivaldi"
	if _, err := os.Stat(ivaldiDir); os.IsNotExist(err) {
		return fmt.Errorf("not in an Ivaldi repository (no .ivaldi directory found)")
	}

	refsManager, err := refs.NewRefsManager(ivaldiDir)
	if err != nil {
		return fmt.Errorf("failed to initialize refs manager: %w", err)
	}
	defer refsManager.Close()

	currentTimeline, _ := refsManager.GetCurrentTimeline()
	if currentTimeline == name {
		return fmt.Errorf("cannot remove current butterfly '%s'. Switch to another timeline first", name)
	}

	objectsDir := filepath.Join(ivaldiDir, "objects")
	casStore, err := cas.NewFileCAS(objectsDir)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}

	mmr, err := history.NewPersistentMMR(casStore, ivaldiDir)
	if err != nil {
		return fmt.Errorf("failed to initialize MMR: %w", err)
	}
	defer mmr.Close()

	bfManager, err := butterfly.NewManager(ivaldiDir, casStore, refsManager, mmr)
	if err != nil {
		return fmt.Errorf("failed to initialize butterfly manager: %w", err)
	}
	defer bfManager.Close()

	if !bfManager.IsButterfly(name) {
		return fmt.Errorf("'%s' is not a butterfly timeline", name)
	}

	children, _ := bfManager.GetChildren(name)

	if len(children) > 0 && !cascadeDelete {
		fmt.Printf("Removing butterfly '%s'...\n", name)
		fmt.Printf("Warning: This butterfly has %d nested butterflies:\n", len(children))
		for _, child := range children {
			fmt.Printf("  - %s\n", child)
		}
		fmt.Println("These will become orphaned. Use --cascade to delete them.")
	}

	err = bfManager.DeleteButterfly(name, cascadeDelete)
	if err != nil {
		return fmt.Errorf("failed to remove butterfly: %w", err)
	}

	fmt.Printf("✓ Removed butterfly '%s'\n", name)

	return nil
}
