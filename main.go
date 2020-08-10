package main

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
)

type Options struct {
	UserAgent        string
	Authorization    string
	CacheOptimalNode bool
}

type LoadBalancer struct {
	Nodes      []*Node
	opt        Options
	CachedNode *Node
}

type F struct {
	Value float32
	Mod   uint8
}

type Node struct {
	Fs         map[string]*F
	URI        string
	LastStatus uint16
}

func NewFrom(nodes []*Node, opt Options) *LoadBalancer {
	return &LoadBalancer{nodes, opt, nil}
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

	req, err := http.NewRequest(http.MethodGet, n.URI, nil)
	if err != nil {
		return err
	}

	resp, err := cl.Do(req)
	if err != nil {
		n.LastStatus = StatusUnavailable
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
		n.LastStatus = StatusUnavailable
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