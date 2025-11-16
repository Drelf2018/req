package req

import (
	"context"
)

// mapCtx 用于存储和获取额外的上下文值
type mapCtx struct {
	context.Context
	m map[string]any
}

// Value 首先尝试从父上下文的 Value 方法获取值，如果未找到，则从当前 mapCtx 的映射中查找键对应的值
func (c *mapCtx) Value(key any) any {
	if v := c.Context.Value(key); v != nil {
		return v
	}
	if k, ok := key.(string); ok {
		return c.m[k]
	}
	return nil
}

// WithMap 创建一个新上下文，用于存储和获取额外的上下文值
//
// parent 为新上下文的父上下文，不能为空
//
// m 为要附加的键值对映射，会被复制到新的上下文中，后续对 m 的修改不会影响上下文中的值
//
// Value 首先尝试从父上下文的 Value 方法获取值，如果未找到，则从当前 mapCtx 的映射中查找键对应的值
func WithMap(parent context.Context, m map[string]any) context.Context {
	if parent == nil {
		panic("req.WithMap: cannot create context from nil parent")
	}
	ctx := &mapCtx{parent, make(map[string]any, len(m))}
	for k, v := range m {
		ctx.m[k] = v
	}
	return ctx
}

// valuesCtx 用于聚合多个上下文的值
type valuesCtx struct {
	context.Context
	values []context.Context
}

// Value 首先从父上下文中查找键对应的值，如果未找到，则依次从子上下文中查找，返回第一个找到的值
func (c *valuesCtx) Value(key any) any {
	if v := c.Context.Value(key); v != nil {
		return v
	}
	for _, ctx := range c.values {
		if v := ctx.Value(key); v != nil {
			return v
		}
	}
	return nil
}

// WithValues 创建一个新的上下文，用于聚合多个上下文的值
//
// parent 为新上下文的父上下文，不能为空
//
// values 为要附加的子上下文列表
//
// Value 首先从父上下文中查找键对应的值，如果未找到，则依次从子上下文中查找，返回第一个找到的值
func WithValues(parent context.Context, values ...context.Context) context.Context {
	if parent == nil {
		panic("req.WithValues: cannot create context from nil parent")
	}
	filtered := make([]context.Context, 0, len(values))
	for _, ctx := range values {
		if ctx != nil {
			filtered = append(filtered, ctx)
		}
	}
	return &valuesCtx{parent, filtered}
}
