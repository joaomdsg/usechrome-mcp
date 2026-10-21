package chrome

import (
	"context"

	"github.com/chromedp/cdproto/accessibility"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/chromedp"

	"github.com/joaomdsg/usechrome-mcp/internal/browser"
)

// Snapshot reads the accessibility tree of a tab's page, or of the element
// req.Selector picks.
func (b *Browser) Snapshot(ctx context.Context, req browser.SnapshotRequest) (browser.Snapshot, error) {
	var snap browser.Snapshot
	err := b.do(ctx, req.Tab, func(tctx context.Context) error {
		doc, err := readDocument(tctx)
		if err != nil {
			return err
		}
		root, err := selectedNode(tctx, req.Selector)
		if err != nil {
			return err
		}
		tree, err := chromedp.Call(tctx, accessibility.GetFullAXTree, accessibility.GetFullAXTreeParams{})
		if err != nil {
			return err
		}
		nodes, err := outline(tree.Nodes, root)
		snap = browser.Snapshot{URL: doc.URL, Title: doc.Title, Nodes: nodes}
		return err
	})
	return snap, err
}

// selectedNode returns the backend id of the element sel picks, or 0 when
// sel is empty.
func selectedNode(ctx context.Context, sel string) (cdp.BackendNodeID, error) {
	if sel == "" {
		return 0, nil
	}
	defer releaseElements(ctx)
	el, err := element(ctx, "", sel)
	if err != nil {
		return 0, err
	}
	res, err := chromedp.Call(ctx, dom.DescribeNode, dom.DescribeNodeParams{ObjectID: el})
	if err != nil {
		return 0, err
	}
	return res.Node.BackendNodeID, nil
}
