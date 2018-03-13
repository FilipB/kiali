// THIS CODE IS PROVIDED AS-IS and the resulting config JSON is targeted for
// standalone Vizceral use.
//
// The following link explains how to run the Vizceral example.  Replace the example
// version of sample_data.json with the configuration generated by this handler
// and you should be able to render the Vizceral service graph.
//
// https://github.com/Netflix/vizceral-example
//
// The following link gives some information about the config format, although
// good documentation on volume handling is hard to find.
//
// https://github.com/Netflix/Vizceral/wiki/How-to-Use#graph-data-format
//
// Algorithm: Walk each tree adding nodes and edges, decorating each with information
//            provided.
//
package vizceral

import (
	"fmt"
	"strings"
	"time"

	"github.com/kiali/swscore/graph/tree"
)

type Metadata struct {
}

type Metrics struct {
	Danger  float64 `json:"danger,omitempty"`
	Warning float64 `json:"warning,omitempty"`
	Normal  float64 `json:"normal,omitempty"`
}

type Connection struct {
	Source   string   `json:"source"`
	Target   string   `json:"target"`
	Metadata Metadata `json:"metadata,omitempty"`
	Metrics  Metrics  `json:"metrics,omitempty"`
}

type Notice struct {
	Title    string `json:"title"`
	Link     string `json:"link,omitempty"`
	Severity int    `json:"severity,omitempty"`
}

type Node struct {
	Renderer    string       `json:"renderer,omitempty"`
	Name        string       `json:"name"`
	DisplayName string       `json:"displayName,omitempty"`
	Class       string       `json:"class,omitempty"`
	Updated     int64        `json:"updated,omitempty"`
	MaxVolume   float64      `json:"maxVolume,omitempty"`
	Metadata    Metadata     `json:"metadata,omitempty"`
	Nodes       []Node       `json:"nodes,omitempty"`
	Connections []Connection `json:"connections,omitempty"`
	Notices     []Notice     `json:"notices,omitempty"`
}

type Config Node

func NewConfig(namespace string, sn *[]tree.ServiceNode) (result Config) {
	namespaceNodes := []Node{}
	var namespaceConnections []Connection
	var maxVolume float64

	for _, t := range *sn {
		walk(&t, &namespaceNodes, &namespaceConnections, &maxVolume)
	}

	regionNamespaceNode := Node{
		Renderer:    "region",
		Name:        namespace,
		Updated:     time.Now().Unix(),
		MaxVolume:   maxVolume,
		Nodes:       namespaceNodes,
		Connections: namespaceConnections,
	}

	regionInternetNode := Node{
		Renderer: "region",
		Name:     "INTERNET",
	}
	regionInternetConnection := Connection{
		Source: "INTERNET",
		Target: namespace,
		Metrics: Metrics{
			// TODO, should break up MaxVolume by code from the actual unknown/ingress nodes
			Normal:  maxVolume * 1.0,
			Warning: maxVolume * 0.0,
			Danger:  maxVolume * 0.0,
		},
	}

	regionNodes := []Node{regionInternetNode, regionNamespaceNode}
	regionConnections := []Connection{regionInternetConnection}

	result = Config{
		Renderer:    "global",
		Name:        "edge",
		Nodes:       regionNodes,
		Connections: regionConnections,
	}
	return result
}

func walk(sn *tree.ServiceNode, nodes *[]Node, connections *[]Connection, volume *float64) {
	// The unknown/unknown root node is set to act as the INTERNET node
	isRoot := "unknown" == sn.Name && "unknown" == sn.Version
	var name string
	if isRoot {
		name = "INTERNET"
	} else {
		name = fmt.Sprintf("%v (%v)", sn.Name, sn.Version)
	}
	_, found := getNode(nodes, name)
	if !found {
		var displayName string
		if isRoot {
			displayName = "INTERNET"
		} else {
			displayName = fmt.Sprintf("%v (%v)", strings.Split(sn.Name, ".")[0], sn.Version)
		}
		n := Node{
			Renderer:    "focusedChild",
			Name:        name,
			DisplayName: displayName,
			Notices: []Notice{
				{
					Title: "Prometheus Graph",
					Link:  sn.Metadata["link_prom_graph"].(string),
				}},
		}
		*nodes = append(*nodes, n)
	}

	var c Connection
	if nil != sn.Parent {
		isParentRoot := "unknown" == sn.Parent.Name && "unknown" == sn.Parent.Version
		var source string
		if isParentRoot {
			source = "INTERNET"
		} else {
			source = fmt.Sprintf("%v (%v)", sn.Parent.Name, sn.Parent.Version)
		}
		c = Connection{
			Source: source,
			Target: name,
			Metrics: Metrics{
				Normal:  sn.Metadata["rate_2xx"].(float64) / (sn.Metadata["rate"].(float64) + 0.0001),
				Warning: sn.Metadata["rate_3xx"].(float64) / (sn.Metadata["rate"].(float64) + 0.0001),
				Danger:  (sn.Metadata["rate_4xx"].(float64) + sn.Metadata["rate_5xx"].(float64)) / (sn.Metadata["rate"].(float64) + 0.0001),
			},
		}

		*volume += sn.Metadata["rate_2xx"].(float64)
		*volume += sn.Metadata["rate_3xx"].(float64)
		*volume += sn.Metadata["rate_4xx"].(float64)
		*volume += sn.Metadata["rate_5xx"].(float64)
	}
	*connections = append(*connections, c)

	for _, child := range sn.Children {
		walk(child, nodes, connections, volume)
	}
}

func getNode(nodes *[]Node, name string) (*Node, bool) {
	for _, n := range *nodes {
		if n.Name == name {
			return &n, true
		}
	}
	return nil, false
}
