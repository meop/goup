package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/meop/golie/layout"
	"github.com/meop/golie/vendored"
	"github.com/meop/golie/wedge"

	"github.com/meop/goup/internal/config"
	"github.com/meop/goup/internal/store"
)

func newPullCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pull",
		Short: "Pull the wedge goup stamps its shims with to its latest",
		Args:  cobra.NoArgs,
		RunE:  runPull,
	}
}

// runPull is the one command that moves the vendored wedge: every other
// uses what is there, and fetches one only where there is none.
func runPull(cmd *cobra.Command, args []string) error {
	unlock, err := config.AcquireLock()
	if err != nil {
		return fail("%v", err)
	}
	defer unlock()
	l, err := store.Layout()
	if err != nil {
		return fail("%v", err)
	}
	dir, r, err := wedgeRelease(l)
	if err != nil {
		return fail("wedge: %v", err)
	}
	from, to, err := r.Refresh(context.Background(), dir, vendored.Options{Fetching: fetchingWedge})
	if err != nil {
		return fail("%s: %v", vendored.Label("wedge"), err)
	}
	printPass("%s", vendored.Said(vendored.Label("wedge"), from, to))
	if from != "" && from != to {
		print("%s: shims keep the wedge that stamped them until [goup sync --reinstall]", vendored.Label("wedge"))
	}
	return nil
}

// syncWedge makes sure goup has its vendored mallet and shim templates: what
// is there is used, with no network, and only where there are none is
// wedge's latest fetched. `pull` is what moves it to a newer release.
func syncWedge(ctx context.Context, l layout.Layout) error {
	dir, r, err := wedgeRelease(l)
	if err != nil {
		return err
	}
	_, err = r.Ensure(ctx, dir, vendored.Options{Fetching: fetchingWedge})
	return err
}

func wedgeRelease(l layout.Layout) (string, vendored.Release, error) {
	dir, err := l.ShimDir()
	if err != nil {
		return "", vendored.Release{}, err
	}
	r, err := wedge.Here()
	return dir, r, err
}

// fetchingWedge says wedge's archive is being fetched, as any other download
// is said
func fetchingWedge(archive string) {
	print("%s: asset fetching [%s]...", vendored.Label("wedge"), archive)
}
