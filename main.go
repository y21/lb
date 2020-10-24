package lb

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

const (
	StatusUnavailable    = uint16(0)
	NodeUnavailableError = "node not available"
	NodeTimeout          = time.Second * 15

	OpUnavailable = 0
	OpAvailable   = 1
)

type Options struct {
	UserAgent        string `json:"userAgent"`
	Authorization    string `json:"authorization"`
	CacheOptimalNode bool   `json:"cacheOptimalNode"`
	Route            string `json:"route"`
}

type UpdateData struct {
	Node *Node
	Op   uint8
}

type LoadBalancer struct {
	Nodes        []*Node `json:"nodes"`
	opt          Options `json:"opt"`
	CachedNode   *Node   `json:"cachedNode"`
	Subscription chan UpdateData
}

type F struct {
	Value float32 `json:"value"`
	Mod   uint8   `json:"mod"`
}

type Node struct {
	Fs         map[string]*F `json:"fs"`
	URI        string        `json:"uri"`
	LastStatus uint16        `json:"lastStatus"`
}

func NewFrom(nodes []*Node, opt Options) *LoadBalancer {
	return &LoadBalancer{nodes, opt, nil, make(chan UpdateData)}
}

func New(opt Options) *LoadBalancer {
	return NewFrom([]*Node{}, opt)
}

func (f *F) GetScore() float32 {
	return f.Value * float32(f.Mod)
}

func (n *Node) IsAvailable() bool {
	return n.LastStatus != StatusUnavailable
}

func (n *Node) IsError() bool {
	return !n.IsAvailable() || n.LastStatus >= 400
}

func (l *LoadBalancer) MakeUnavailableNode(n *Node, status uint16) {
	n.LastStatus = status

	l.Subscription <- UpdateData{
		Node: n,
		Op:   OpUnavailable,
	}
}

func (n *Node) GetScore() (sum float32) {
	for _, f := range n.Fs {
		sum += f.GetScore()
	}
	return sum
}

func (lb *LoadBalancer) PingNode(n *Node) error {
	if n == nil {
		return errors.New("node is nil")
	}

	cl := &http.Client{
		Timeout: NodeTimeout,
	}

	req, err := http.NewRequest(http.MethodGet, n.URI+lb.opt.Route, nil)
	if err != nil {
		return err
	}

	if lb.opt.Authorization != "" {
		req.Header.Set("Authorization", lb.opt.Authorization)
	}

	resp, err := cl.Do(req)
	if err != nil {
		lb.MakeUnavailableNode(n, StatusUnavailable)
		return errors.New(NodeUnavailableError)
	}
	defer resp.Body.Close()

	n.LastStatus = uint16(resp.StatusCode)

	if resp.StatusCode >= http.StatusBadRequest {
		return errors.New(NodeUnavailableError)
	}

	body := make(map[string]float32)
	err = json.NewDecoder(resp.Body).Decode(&body)
	if err != nil {
		lb.MakeUnavailableNode(n, StatusUnavailable)
		return errors.New(NodeUnavailableError)
	}

	for k, v := range body {
		f, ok := n.Fs[k]
		if ok {
			f.Value = v
		}
	}

	return nil
}

func (lb *LoadBalancer) Ping() {
	for _, v := range lb.Nodes {
		lb.PingNode(v)

		if !v.IsError() && lb.opt.CacheOptimalNode {
			if lb.CachedNode == nil || v.GetScore() < lb.CachedNode.GetScore() {
				lb.CachedNode = v
			}
		}
	}
}

func (lb *LoadBalancer) Watch(d time.Duration) {
	for {
		lb.Ping()
		time.Sleep(d)
	}
}

func (lb *LoadBalancer) GetOptimalNode(onlyAvailable bool) (n *Node) {
	if lb.CachedNode != nil {
		return lb.CachedNode
	}

	for _, v := range lb.Nodes {
		if n == nil || v.GetScore() < n.GetScore() {
			if onlyAvailable && v.IsError() {
				continue
			}

			n = v
		}
	}

	return n
}
