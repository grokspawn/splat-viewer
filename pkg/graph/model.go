package graph

type Node struct {
	ID                  string   `json:"id"`
	Package             string   `json:"package"`
	BundleName          string   `json:"bundleName"`
	Version             string   `json:"version"`
	Release             string   `json:"release"`
	Channels            []string `json:"channels"`
	IsChannelHead       bool     `json:"isChannelHead"`
	Skipped             bool     `json:"skipped"`
	Deprecated          bool     `json:"deprecated"`
	DeprecationMessage  string   `json:"deprecationMessage,omitempty"`
	MaxOpenShiftVersion string   `json:"maxOpenShiftVersion,omitempty"`
	Group               string   `json:"group"`
	Phantom             bool     `json:"phantom,omitempty"`
	FX                  float64  `json:"fx"`
	FY                  float64  `json:"fy"`
	FZ                  float64  `json:"fz"`
}

type Link struct {
	Source  string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"`
	Channel string `json:"channel,omitempty"`
	Label  string `json:"label,omitempty"`
}

type BuildOptions struct {
	IncludeSkipRangeEdges bool
}

type Graph struct {
	Catalog   string   `json:"catalog"`
	Releases  []string `json:"releases"`
	Generated string   `json:"generated"`
	Nodes     []Node   `json:"nodes"`
	Links     []Link   `json:"links"`
}
