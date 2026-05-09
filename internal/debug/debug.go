package debug

import (
	"context"
	"fmt"
	"sync"
)

type debugKey struct{}

type DebugVar struct {
	Key   string
	Value string
}

type DebugInfo struct {
	Mu           sync.Mutex
	Template     string
	Queries      []string
	Vars         []DebugVar
	TemplateData string
}

func FromContext(ctx context.Context) *DebugInfo {
	v, _ := ctx.Value(debugKey{}).(*DebugInfo)
	return v
}

func NewContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, debugKey{}, &DebugInfo{})
}

func RecordQuery(ctx context.Context, query string) {
	di := FromContext(ctx)
	if di == nil {
		return
	}
	di.Mu.Lock()
	di.Queries = append(di.Queries, query)
	di.Mu.Unlock()
}

func RecordVar(ctx context.Context, key string, value any) {
	di := FromContext(ctx)
	if di == nil {
		return
	}
	di.Mu.Lock()
	di.Vars = append(di.Vars, DebugVar{Key: key, Value: fmt.Sprintf("%v", value)})
	di.Mu.Unlock()
}

func RecordTemplateData(ctx context.Context, data string) {
	di := FromContext(ctx)
	if di == nil {
		return
	}
	di.Mu.Lock()
	di.TemplateData = data
	di.Mu.Unlock()
}
