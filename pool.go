package req

import (
	"context"
	"errors"
	"io"
	"net/http"
	"sync"
	"time"
)

// task 任务包含请求和响应
type task struct {
	API
	response *http.Response
	content  []byte
	err      error
	wg       sync.WaitGroup
}

// Response 等待任务完成后获取响应
func (t *task) Response() (*http.Response, error) {
	t.wg.Wait()
	return t.response, t.err
}

// Content 等待任务完成后获取响应体
func (t *task) Content() ([]byte, error) {
	t.wg.Wait()
	return t.content, t.err
}

// Text 等待任务完成后获取响应体字符串
func (t *task) Text() (string, error) {
	t.wg.Wait()
	return string(t.content), t.err
}

// DefaultPool 默认请求池
var DefaultPool *Pool

// Pool 请求池
type Pool struct {
	// 执行请求的最大并行数
	Machine int

	// 请求池缓冲大小
	Size int

	// 请求超时
	Timeout time.Duration

	// 发送请求的会话
	Session *Session

	// 请求调度器
	processor chan *task

	// 管理机器
	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc
}

// Run 启动请求池
func (p *Pool) Run() {
	p.RunWithContext(context.Background())
}

// RunWithContext 携带上下文启动请求池
func (p *Pool) RunWithContext(ctx context.Context) {
	// 避免重复启动
	if p.ctx != nil || p.cancel != nil {
		return
	}
	p.ctx, p.cancel = context.WithCancel(ctx)
	// 设置默认值，保证零值可用
	if p.Machine <= 0 {
		p.Machine = 4
	}
	if p.Size <= 0 {
		p.Size = 256
	}
	if p.Timeout <= 0 {
		p.Timeout = 60 * time.Second
	}
	if p.Session == nil {
		p.Session = DefaultSession
	}
	// 开启发送请求
	p.processor = make(chan *task, p.Size)
	p.wg.Add(p.Machine)
	for i := 0; i < p.Machine; i++ {
		go func() {
			defer p.wg.Done()
			for {
				select {
				case <-p.ctx.Done():
					return
				case task := <-p.processor:
					timeout, cancel := context.WithTimeout(p.ctx, p.Timeout)
					task.response, task.err = p.Session.DoWithContext(timeout, task.API)
					if task.err == nil {
						task.content, task.err = io.ReadAll(task.response.Body)
						task.response.Body.Close()
					}
					task.wg.Done()
					cancel()
				}
			}
		}()
	}
}

// ErrPoolClosed 请求池已经关闭
var ErrPoolClosed = errors.New("req: pool has been closed")

// NewTask 用请求创建任务
func (p *Pool) NewTask(api API) *task {
	if p.cancel == nil {
		return &task{err: ErrPoolClosed}
	}
	task := &task{API: api}
	task.wg.Add(1)
	p.processor <- task
	return task
}

// Shutdown 关闭请求池，会等待所有已发送的请求，并且为未发送的请求设置错误
func (p *Pool) Shutdown() {
	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
		p.wg.Wait()
		close(p.processor)
		for task := range p.processor {
			task.err = ErrPoolClosed
			task.wg.Done()
		}
		p.ctx = nil
	}
}
