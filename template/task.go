package template

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/cookiejar"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"text/template"
	"time"
	"unicode"

	"github.com/mattn/go-shellwords"
	"gopkg.in/yaml.v3"
)

// EnvKey 用来从上下文获取环境变量映射的键
//
//	env, ok := ctx.Value(EnvKey).(map[string]any)
var EnvKey string = "__env__"

// ErrLoadEnv 加载环境变量错误
var ErrLoadEnv = errors.New("cannot load env from nil context")

// LoadEnv 从上下文获取环境变量映射的克隆
func LoadEnv(ctx context.Context) map[string]any {
	if ctx == nil {
		panic(ErrLoadEnv)
	}
	if env, ok := ctx.Value(EnvKey).(map[string]any); ok && len(env) != 0 {
		return maps.Clone(env)
	}
	return make(map[string]any)
}

// ExecuteOutputs 执行模板获取值并写入输出映射
func ExecuteOutputs(tmpl *template.Template, data any, inputs, outputs map[string]any) (err error) {
	// 先获取模板的克隆，避免保存函数产生副作用，或者在未提供模板时创建新的
	if tmpl == nil {
		tmpl = NewTemplate("")
	} else {
		tmpl, err = tmpl.Clone()
		if err != nil {
			return err
		}
	}
	tmpl.Funcs(template.FuncMap{"save": func(key string, value any) string {
		outputs[key] = value
		return ""
	}})
	// 收集所有要写入的键，当输入和输出一样时，不可以一边遍历一边写入
	// 当键名以特殊符号开头时，会将其值计算的结果直接写入输出
	vars := make([]string, 0, len(inputs))
	keys := make([]string, 0, len(inputs))
	for key := range inputs {
		if strings.HasPrefix(key, "$") {
			vars = append(vars, key)
		} else {
			keys = append(keys, key)
		}
	}
	// 遍历所有普通键名，对于非字符串值直接写入输出，否则通过模板写入要输出的值
	for _, key := range keys {
		if v, ok := inputs[key].(string); ok {
			_, err = tmpl.Parse(v)
			if err != nil {
				return fmt.Errorf("input %q: %w", key, err)
			}
			buf := &bytes.Buffer{}
			err = tmpl.Execute(buf, data)
			if err != nil {
				return fmt.Errorf("input %q: %w", key, err)
			}
			outputs[key] = buf.String()
		} else {
			outputs[key] = inputs[key]
		}
	}
	// 遍历所有特殊键名，其对应的普通键名的值会被覆盖
	for _, key := range vars {
		if v, ok := inputs[key].(string); ok {
			_, err = tmpl.Parse(fmt.Sprintf("{{ %s | save %q }}", v, key[1:]))
			if err != nil {
				return fmt.Errorf("input %q: %w", key, err)
			}
			err = tmpl.Execute(io.Discard, data)
			if err != nil {
				return fmt.Errorf("input %q: %w", key, err)
			}
		} else {
			return fmt.Errorf("input %q: $-key value must be a template string, got %T", key, inputs[key])
		}
	}
	return nil
}

// UpdateEnv 更新上下文和模板中的环境变量映射
//
// 先对模板进行克隆，此时可以使用原先的环境变量函数，还可以为其设置新函数而不影响旧模板。
// 接着从上下文获取到旧的环境变量映射的克隆，在对其更新后作为新的环境变量映射。
// 对要添加的环境变量进行计算后写入新映射，再将新映射绑定在要返回的上下文和模板上。
func UpdateEnv(ctx context.Context, tmpl *template.Template, data any, env map[string]any) (context.Context, *template.Template, error) {
	if ctx == nil {
		return nil, nil, ErrLoadEnv
	}
	if tmpl == nil {
		tmpl = NewTemplate("")
	} else {
		var err error
		tmpl, err = tmpl.Clone()
		if err != nil {
			return nil, nil, err
		}
	}
	o := LoadEnv(ctx)
	err := ExecuteOutputs(tmpl, data, env, o)
	if err != nil {
		return nil, nil, err
	}
	return context.WithValue(ctx, EnvKey, o), tmpl.Funcs(template.FuncMap{"env": func() map[string]any { return o }}), nil
}

// GoodName 判断 ID 是否合法
var GoodName = func(name string) error {
	if name == "" {
		return fmt.Errorf("name %q: empty identifier", name)
	}
	for i, r := range name {
		switch {
		case r == '_':
		case i == 0 && !unicode.IsLetter(r):
			return fmt.Errorf("name %q: invalid identifier", name)
		case !unicode.IsLetter(r) && !unicode.IsDigit(r):
			return fmt.Errorf("name %q: invalid identifier", name)
		}
	}
	return nil
}

// UnmarshalAction 反序列化可复用动作
var UnmarshalAction = func(name string) (*Action, error) {
	b, err := os.ReadFile(name)
	if err != nil {
		return nil, err
	}
	var a Action
	return &a, yaml.Unmarshal(b, &a)
}

// UnmarshalWorkflow 反序列化工作流
var UnmarshalWorkflow = func(name string) (*Workflow, error) {
	b, err := os.ReadFile(name)
	if err != nil {
		return nil, err
	}
	var w Workflow
	return &w, yaml.Unmarshal(b, &w)
}

// Step 是包含请求信息的步骤
type Step struct {
	ID      string         `yaml:"id,omitempty"`      // 步骤标识符
	If      string         `yaml:"if,omitempty"`      // 执行条件
	Name    string         `yaml:"name,omitempty"`    // 步骤名称
	CURL    string         `yaml:"curl,omitempty"`    // 请求命令
	Uses    string         `yaml:"uses,omitempty"`    // 引用动作
	Env     map[string]any `yaml:"env,omitempty"`     // 环境变量
	With    map[string]any `yaml:"with,omitempty"`    // 输入参数
	Outputs map[string]any `yaml:"outputs,omitempty"` // 输出参数
	Timeout int            `yaml:"timeout,omitempty"` // 超时秒数

	client   *http.Client       // 请求客户端
	template *template.Template // 步骤模板

	response *http.Response // 请求响应
	content  []byte         // 响应内容
	result   any            // 响应结果
}

// reset 清除残留
func (s *Step) reset() {
	s.client = nil
	s.template = nil
}

// Do 发送请求
func (s *Step) Do(ctx context.Context) (err error) {
	// 初始化
	s.response = nil
	s.content = nil
	s.result = nil
	defer s.reset()
	// 加载环境变量
	ctx, s.template, err = UpdateEnv(ctx, s.template, s, s.Env)
	if err != nil {
		return fmt.Errorf("step %q load env: %w", s.Name, err)
	}
	// 模板填充
	_, err = s.template.Parse(s.CURL)
	if err != nil {
		return fmt.Errorf("step %q curl %q: %w", s.Name, s.CURL, err)
	}
	buf := &bytes.Buffer{}
	err = s.template.Execute(buf, s)
	if err != nil {
		return fmt.Errorf("step %q curl %q: %w", s.Name, s.CURL, err)
	}
	// 解析命令
	curl := buf.String()
	args, err := shellwords.Parse(curl)
	if err != nil {
		return fmt.Errorf("step %q curl %q: %w", s.Name, curl, err)
	}
	// 设置超时
	var timeout time.Duration
	if s.Timeout == 0 {
		timeout = time.Minute
	} else {
		timeout = time.Duration(s.Timeout) * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	// 构造请求
	r, err := argsToRequest(ctx, args...)
	if err != nil {
		return fmt.Errorf("step %q: %w", s.Name, err)
	}
	// 发送请求
	if s.client != nil {
		s.response, err = s.client.Do(r)
	} else {
		s.response, err = http.DefaultClient.Do(r)
	}
	if err != nil {
		return fmt.Errorf("step %q: %w", s.Name, err)
	}
	defer s.response.Body.Close()
	if s.response.StatusCode != http.StatusOK {
		io.Copy(io.Discard, s.response.Body)
		return fmt.Errorf("step %q: response status not ok: %s", s.Name, s.response.Status)
	}
	// 获取响应体
	s.content, err = io.ReadAll(s.response.Body)
	if err != nil {
		return fmt.Errorf("step %q: read response body: %w", s.Name, err)
	}
	// 写入输出
	s.template.Funcs(template.FuncMap{
		"response": s.Response,
		"content":  s.Content,
		"result":   s.Result,
		"text":     s.Text,
	})
	err = ExecuteOutputs(s.template, s, s.Outputs, s.Outputs)
	if err != nil {
		return fmt.Errorf("step %q %w", s.Name, err)
	}
	return nil
}

// Response 在发送请求成功后获取响应
func (s *Step) Response() (*http.Response, error) {
	if s.response == nil {
		return nil, fmt.Errorf("step %q: response not yet available", s.Name)
	}
	return s.response, nil
}

// Content 在发送请求成功后获取响应体
func (s *Step) Content() ([]byte, error) {
	if s.content == nil {
		return nil, fmt.Errorf("step %q: content not yet available", s.Name)
	}
	return s.content, nil
}

// Result 在发送请求成功后将响应体反序列化
func (s *Step) Result() (any, error) {
	if s.result == nil {
		b, err := s.Content()
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(b, &s.result)
		if err != nil {
			return nil, fmt.Errorf("step %q: unmarshal response body: %w", s.Name, err)
		}
	}
	return s.result, nil
}

// Text 在发送请求成功后获取响应体字符串
func (s *Step) Text() (string, error) {
	b, err := s.Content()
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Use 执行步骤调用的可复用动作
func (s *Step) Use(ctx context.Context) error {
	defer s.reset()
	// 反序列化可复用动作
	a, err := UnmarshalAction(s.Uses)
	if err != nil {
		return fmt.Errorf("step %q uses %q: %w", s.Name, s.Uses, err)
	}
	if a == nil {
		return fmt.Errorf("step %q uses %q: nil action", s.Name, s.Uses)
	}
	// 加载环境变量
	_, s.template, err = UpdateEnv(ctx, s.template, s, s.Env)
	if err != nil {
		return fmt.Errorf("step %q load env: %w", s.Name, err)
	}
	// 写入入参
	with := make(map[string]any, len(s.With))
	err = ExecuteOutputs(s.template, s, s.With, with)
	if err != nil {
		return fmt.Errorf("step %q load with: %w", s.Name, err)
	}
	// 调用动作，传入清除环境变量的上下文
	s.Outputs, err = a.Call(context.WithValue(ctx, EnvKey, nil), with)
	if err != nil {
		return fmt.Errorf("step %q %w", s.Name, err)
	}
	return nil
}

// wrapError 合并错误
func wrapError(errs ...error) (e error) {
	for _, err := range errs {
		if err != nil {
			if e == nil {
				e = err
			} else {
				e = fmt.Errorf("%w\n\nDuring handling of the above error, another error occurred:\n\n%w", e, err)
			}
		}
	}
	return
}

// NormalizeUses 归一化 uses 标识，用于环检测中的节点判等
//
// 默认反序列化实现从文件读取，因此默认按文件路径归一化，以合并
// 同一文件的不同写法。若替换了 UnmarshalAction 与 UnmarshalWorkflow，
// 请一并替换本函数以匹配新的标识语义。
var NormalizeUses = func(uses string) string {
	if abs, err := filepath.Abs(uses); err == nil {
		return filepath.Clean(abs)
	}
	return filepath.Clean(uses)
}

// graph 有向依赖图
type graph struct {
	indegree map[string]int
	next     map[string][]string
}

// newGraph 新建有向依赖图
func newGraph() *graph {
	return &graph{
		indegree: make(map[string]int),
		next:     make(map[string][]string),
	}
}

// add 确保节点存在
func (g *graph) add(id string) {
	if _, ok := g.indegree[id]; !ok {
		g.indegree[id] = 0
	}
}

// edge 添加依赖边：to 依赖 from
func (g *graph) edge(from, to string) {
	g.add(from)
	g.add(to)
	g.indegree[to]++
	g.next[from] = append(g.next[from], to)
}

// cycle 用 Kahn 算法检测环，返回环上节点（已排序），无环返回 nil
func (g *graph) cycle() []string {
	queue := make([]string, 0, len(g.indegree))
	for id, degree := range g.indegree {
		if degree == 0 {
			queue = append(queue, id)
		}
	}
	head := 0
	for ; head < len(queue); head++ {
		id := queue[head]

		for _, to := range g.next[id] {
			g.indegree[to]--
			if g.indegree[to] == 0 {
				queue = append(queue, to)
			}
		}
	}
	if head == len(g.indegree) {
		return nil
	}
	cycle := make([]string, 0)
	for id, degree := range g.indegree {
		if degree > 0 {
			cycle = append(cycle, id)
		}
	}
	sort.Strings(cycle)
	return cycle
}

// Steps 是步骤集
type Steps []*Step

// CheckUses 检测自身步骤引用的动作链
func (s Steps) CheckUses() error {
	// 收集所有步骤的 uses 作为起点
	queue := make([]string, 0, len(s))
	for _, step := range s {
		if strings.TrimSpace(step.Uses) != "" {
			queue = append(queue, step.Uses)
		}
	}
	if len(queue) == 0 {
		return nil
	}
	// 在一张图上检测环，共享 visited 避免重复加载
	g := newGraph()
	visited := make(map[string]bool)
	for head := 0; head < len(queue); head++ {
		node := queue[head]
		key := NormalizeUses(node)
		if visited[key] {
			continue
		}
		visited[key] = true
		g.add(key)
		a, err := UnmarshalAction(node)
		if err != nil {
			return fmt.Errorf("uses %q: %w", node, err)
		}
		if a == nil {
			return fmt.Errorf("uses %q: nil action", node)
		}
		for _, step := range a.Steps {
			if strings.TrimSpace(step.Uses) != "" {
				g.edge(key, NormalizeUses(step.Uses))
				queue = append(queue, step.Uses)
			}
		}
	}
	if cycle := g.cycle(); len(cycle) != 0 {
		return fmt.Errorf("uses cycle not allowed: %s", strings.Join(cycle, ", "))
	}
	return nil
}

// Run 顺序执行步骤
func (s Steps) Run(ctx context.Context, tmpl *template.Template) (stepsErr error) {
	if ctx == nil {
		return ErrLoadEnv
	}
	if tmpl == nil {
		tmpl = NewTemplate("")
	}
	// 构建步骤映射，并绑定在原始模板上
	steps := make(map[string]*Step, len(s))
	for _, step := range s {
		if step.ID != "" {
			err := GoodName(step.ID)
			if err != nil {
				return fmt.Errorf("step %q: %w", step.Name, err)
			}
			_, exists := steps[step.ID]
			if exists {
				return fmt.Errorf("step %q: duplicate id %q", step.Name, step.ID)
			}
			steps[step.ID] = step
		}
		if step.CURL == "" && step.Uses == "" {
			return fmt.Errorf("step %q: must define a 'curl' or 'uses' key", step.Name)
		}
		if strings.TrimSpace(step.CURL) != "" && strings.TrimSpace(step.Uses) != "" {
			return fmt.Errorf("step %q: the 'curl' and 'uses' keys are mutually exclusive", step.Name)
		}
		if step.Outputs == nil {
			step.Outputs = make(map[string]any)
		}
	}
	tmpl.Funcs(template.FuncMap{"steps": func() map[string]*Step { return steps }})
	// 创建克隆模板，绑定条件判断函数，避免对原始模板产生副作用
	tmpl, stepsErr = tmpl.Clone()
	if stepsErr != nil {
		return
	}
	tmpl.Funcs(template.FuncMap{
		"always":    func() bool { return true },
		"success":   func() bool { return stepsErr == nil && ctx.Err() == nil },
		"failure":   func() bool { return stepsErr != nil && ctx.Err() == nil },
		"cancelled": func() bool { return ctx.Err() != nil },
	})
	// 同一步骤集的步骤共用一个客户端
	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Transport:     http.DefaultClient.Transport,
		Timeout:       http.DefaultClient.Timeout,
		CheckRedirect: http.DefaultClient.CheckRedirect,
		Jar:           jar,
	}
	// 遍历步骤并发送请求，每个步骤的执行结果分为成功、失败和被取消
	for idx, step := range s {
		// 为步骤创建子模板
		if step.ID != "" {
			step.template = tmpl.New(fmt.Sprintf("%s.steps.%s", tmpl.Name(), step.ID))
		} else {
			step.template = tmpl.New(fmt.Sprintf("%s.steps.%d", tmpl.Name(), idx))
		}
		// 判断是否执行步骤
		cond := strings.TrimSpace(step.If)
		if cond == "" {
			cond = "success"
		}
		ok, err := ToValue[bool](step.template, cond, s)
		if err != nil {
			stepsErr = wrapError(stepsErr, fmt.Errorf("step %q if %q: %w", step.Name, step.If, err))
			continue
		}
		if !ok {
			continue
		}
		// 当上下文已经关闭但仍需要执行，表示这里是错误处理步骤，将上下文包装成不关闭的上下文
		stepCtx := ctx
		if ctx.Err() != nil {
			stepCtx = context.WithoutCancel(ctx)
		}
		// 分情况执行步骤，执行可复用动作或者直接发送请求
		if step.Uses != "" {
			if err = step.Use(stepCtx); err != nil {
				stepsErr = wrapError(stepsErr, err)
				continue
			}
		} else {
			step.client = client
			if err = step.Do(stepCtx); err != nil {
				stepsErr = wrapError(stepsErr, err)
				continue
			}
		}
		// 将最后一个成功步骤的输出设为快捷函数，只有不是第一个的步骤可用，隔离外部
		tmpl.Funcs(template.FuncMap{"outputs": func() map[string]any { return step.Outputs }})
	}
	return
}

// Input 是可复用动作、工作流的输入
type Input struct {
	Description string `yaml:"description,omitempty"` // 输入描述
	Required    bool   `yaml:"required,omitempty"`    // 是否必填
	Type        string `yaml:"type,omitempty"`        // 输入类型
	Default     any    `yaml:"default,omitempty"`     // 默认值
}

// Verify 校验输入
//
// 没有输入时，如果是必填项则报错，否则检查是否提供默认值，如果没有提供默认值，则返回空字符串。
// 如果提供了默认值，则设置输入为默认值，并与有输入的情况一同匹配类型是否正确。
func (i Input) Verify(v any) (any, error) {
	if v == nil {
		if i.Required {
			return nil, fmt.Errorf("not provided")
		}
		if i.Default == nil {
			return "", nil
		}
		v = i.Default
	}
	if i.Type != fmt.Sprintf("%T", v) {
		return nil, fmt.Errorf(`want "%s", got "%T"`, i.Type, v)
	}
	return v, nil
}

// Output 是可复用动作、工作流的输出
type Output struct {
	Description string `yaml:"description,omitempty"` // 输出描述
	Value       any    `yaml:"value,omitempty"`       // 输出值
}

// Action 是可复用动作
type Action struct {
	Name        string            `yaml:"name,omitempty"`        // 动作名称
	Description string            `yaml:"description,omitempty"` // 动作描述
	Author      string            `yaml:"author,omitempty"`      // 动作作者
	Inputs      map[string]Input  `yaml:"inputs,omitempty"`      // 动作输入
	Outputs     map[string]Output `yaml:"outputs,omitempty"`     // 动作输出
	Steps       Steps             `yaml:"steps,omitempty"`       // 动作步骤
}

// Call 调用动作
func (a *Action) Call(ctx context.Context, with map[string]any) (map[string]any, error) {
	// 校验输入
	var err error
	inputs := make(map[string]any, len(a.Inputs))
	for key, input := range a.Inputs {
		inputs[key], err = input.Verify(with[key])
		if err != nil {
			return map[string]any{}, fmt.Errorf("action %q input %q: %w", a.Name, key, err)
		}
	}
	tmpl := NewTemplate("").Funcs(template.FuncMap{"inputs": func() map[string]any { return inputs }})
	// 顺序执行步骤
	err = a.Steps.Run(ctx, tmpl)
	if err != nil {
		return map[string]any{}, fmt.Errorf("action %q %w", a.Name, err)
	}
	// 写入输出
	outputs := make(map[string]any, len(a.Outputs))
	for key, output := range a.Outputs {
		outputs[key] = output.Value
	}
	err = ExecuteOutputs(tmpl, a, outputs, outputs)
	if err != nil {
		return map[string]any{}, fmt.Errorf("action %q %w", a.Name, err)
	}
	return outputs, nil
}

// Needs 是 GitHub Actions 中的 needs 字段类型
type Needs []string

// UnmarshalYAML 实现 Needs 的单字符串和字符串列表读取方式
func (n *Needs) UnmarshalYAML(value *yaml.Node) error {
	// 处理空节点
	if value.Kind == yaml.ScalarNode && value.Value == "" {
		*n = nil
		return nil
	}
	// 根据节点类型分发
	switch value.Kind {
	case yaml.ScalarNode:
		// 单个字符串转为单元素列表
		*n = []string{value.Value}
		return nil
	case yaml.SequenceNode:
		// 字符串列表逐个解析
		var items []string
		for _, item := range value.Content {
			var s string
			if err := item.Decode(&s); err != nil {
				return fmt.Errorf("invalid need: %w", err)
			}
			items = append(items, s)
		}
		*n = items
		return nil
	default:
		return fmt.Errorf("invalid need type: %v", value.Kind)
	}
}

var _ yaml.Unmarshaler = (*Needs)(nil)

// Job 是包含多个顺序执行步骤的任务
type Job struct {
	If      string         `yaml:"if,omitempty"`      // 执行条件
	Name    string         `yaml:"name,omitempty"`    // 任务名称
	Needs   Needs          `yaml:"needs,omitempty"`   // 依赖任务
	Uses    string         `yaml:"uses,omitempty"`    // 引用工作流
	Env     map[string]any `yaml:"env,omitempty"`     // 环境变量
	With    map[string]any `yaml:"with,omitempty"`    // 输入参数
	Outputs map[string]any `yaml:"outputs,omitempty"` // 输出参数
	Timeout int            `yaml:"timeout,omitempty"` // 超时秒数
	Steps   Steps          `yaml:"steps,omitempty"`   // 任务步骤

	template *template.Template // 任务模板

	err        error        // 任务执行错误
	success    bool         // 任务完成情况
	counter    atomic.Int32 // 剩余依赖计数
	downstream []string     // 下游任务列表
}

// Result 任务执行结果
//
//	"success" // 执行成功
//	"failure" // 执行失败（任意 step 失败，或超时等错误）
//	"skipped" // 被跳过（if 条件为 false，或它依赖的任务没有成功）
func (j *Job) Result() string {
	switch {
	case j.success:
		return "success"
	case j.err != nil:
		return "failure"
	default:
		return "skipped"
	}
}

// Do 执行任务
func (j *Job) Do(ctx context.Context) (err error) {
	// 加载环境变量
	ctx, j.template, err = UpdateEnv(ctx, j.template, j, j.Env)
	if err != nil {
		return fmt.Errorf("job %q load env: %w", j.Name, err)
	}
	// 设置超时
	var timeout time.Duration
	if j.Timeout == 0 {
		timeout = time.Duration(len(j.Steps)) * time.Minute
	} else {
		timeout = time.Duration(j.Timeout) * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	// 顺序执行步骤
	err = j.Steps.Run(ctx, j.template)
	if err != nil {
		return fmt.Errorf("job %q %w", j.Name, err)
	}
	// 写入输出
	err = ExecuteOutputs(j.template, j, j.Outputs, j.Outputs)
	if err != nil {
		return fmt.Errorf("job %q %w", j.Name, err)
	}
	return nil
}

// Use 执行任务调用的工作流
func (j *Job) Use(ctx context.Context) error {
	// 反序列化工作流
	w, err := UnmarshalWorkflow(j.Uses)
	if err != nil {
		return fmt.Errorf("job %q uses %q: %w", j.Name, j.Uses, err)
	}
	if w == nil {
		return fmt.Errorf("job %q uses %q: nil workflow", j.Name, j.Uses)
	}
	// 加载环境变量
	_, j.template, err = UpdateEnv(ctx, j.template, j, j.Env)
	if err != nil {
		return fmt.Errorf("job %q load env: %w", j.Name, err)
	}
	// 写入入参
	with := make(map[string]any, len(j.With))
	err = ExecuteOutputs(j.template, j, j.With, with)
	if err != nil {
		return fmt.Errorf("job %q load with: %w", j.Name, err)
	}
	// 调用工作流，传入清除环境变量的上下文
	j.Outputs, err = w.Call(context.WithValue(ctx, EnvKey, nil), with)
	if err != nil {
		return fmt.Errorf("job %q %w", j.Name, err)
	}
	return nil
}

// Workflow 是包含多个同时执行任务的工作流
type Workflow struct {
	Name    string            `yaml:"name,omitempty"`    // 工作流名称
	Inputs  map[string]Input  `yaml:"inputs,omitempty"`  // 工作流输入
	Outputs map[string]Output `yaml:"outputs,omitempty"` // 工作流输出
	Env     map[string]any    `yaml:"env,omitempty"`     // 环境变量
	Jobs    map[string]*Job   `yaml:"jobs,omitempty"`    // 工作流任务

	template *template.Template // 工作流模板

	ready chan string    // 待执行任务 ID
	wg    sync.WaitGroup // 等待所有任务完成
}

// CheckNeeds 检测当前工作流任务之间的依赖环与未知依赖
func (w *Workflow) CheckNeeds() error {
	g := newGraph()
	for id, job := range w.Jobs {
		g.add(id)
		seen := make(map[string]bool)
		for _, need := range job.Needs {
			if seen[need] {
				continue
			}
			seen[need] = true
			if _, ok := w.Jobs[need]; !ok {
				return fmt.Errorf("workflow %q job %q: depends on unknown job %q", w.Name, id, need)
			}
			g.edge(need, id)
		}
	}
	if cycle := g.cycle(); len(cycle) != 0 {
		return fmt.Errorf("workflow %q: needs cycle not allowed: %s", w.Name, strings.Join(cycle, ", "))
	}
	return nil
}

// CheckUses 检测当前工作流的任务依赖链、任务引用的工作流链以及任务步骤引用的动作链
func (w *Workflow) CheckUses() error {
	if err := w.CheckNeeds(); err != nil {
		return err
	}
	// 收集所有任务的 uses 作为起点
	queue := make([]string, 0, len(w.Jobs))
	for _, job := range w.Jobs {
		if strings.TrimSpace(job.Uses) != "" {
			queue = append(queue, job.Uses)
		} else if err := job.Steps.CheckUses(); err != nil {
			return fmt.Errorf("workflow %q job %q %w", w.Name, job.Name, err)
		}
	}
	if len(queue) == 0 {
		return nil
	}
	// 在一张图上检测环，共享 visited 避免重复加载
	g := newGraph()
	visited := make(map[string]bool)
	for head := 0; head < len(queue); head++ {
		node := queue[head]
		key := NormalizeUses(node)
		if visited[key] {
			continue
		}
		visited[key] = true
		g.add(key)
		nw, err := UnmarshalWorkflow(node)
		if err != nil {
			return fmt.Errorf("uses %q: %w", node, err)
		}
		if nw == nil {
			return fmt.Errorf("uses %q: nil workflow", node)
		}
		if err := nw.CheckNeeds(); err != nil {
			return fmt.Errorf("uses %q %w", node, err)
		}
		for _, job := range nw.Jobs {
			if strings.TrimSpace(job.Uses) != "" {
				g.edge(key, NormalizeUses(job.Uses))
				queue = append(queue, job.Uses)
			} else if err := job.Steps.CheckUses(); err != nil {
				return fmt.Errorf("workflow %q job %q: %w", nw.Name, job.Name, err)
			}
		}
	}
	if cycle := g.cycle(); len(cycle) != 0 {
		return fmt.Errorf("workflow %q: uses cycle not allowed: %s", w.Name, strings.Join(cycle, ", "))
	}
	return nil
}

// CheckKey 用来从上下文获取工作流是否已经检测过环
//
//	checked, ok := ctx.Value(CheckKey).(bool)
var CheckKey string = "__check__"

// Do 启动调度，参数 workerNum 为并发数
func (w *Workflow) Do(ctx context.Context, workerNum int) error {
	if workerNum <= 0 {
		return fmt.Errorf("workflow %q: workerNum must be positive", w.Name)
	}

	// 检查工作流内是否成环
	if checked, ok := ctx.Value(CheckKey).(bool); !checked || !ok {
		if err := w.CheckUses(); err != nil {
			return err
		}
		ctx = context.WithValue(ctx, CheckKey, true)
	}

	// 重置任务状态
	for id, job := range w.Jobs {
		err := GoodName(id)
		if err != nil {
			return fmt.Errorf("workflow %q: %w", w.Name, err)
		}
		if job.Uses == "" && len(job.Steps) == 0 {
			return fmt.Errorf("workflow %q job %q: must define a 'uses' or 'steps' key", w.Name, job.Name)
		}
		if strings.TrimSpace(job.Uses) != "" && len(job.Steps) != 0 {
			return fmt.Errorf("workflow %q job %q: the 'uses' and 'steps' keys are mutually exclusive", w.Name, job.Name)
		}
		job.err = nil
		job.success = false
		job.downstream = nil
		job.template = nil
		if job.Outputs == nil {
			job.Outputs = make(map[string]any)
		}
	}

	// 构建反向依赖表
	for id, job := range w.Jobs {
		seen := make(map[string]bool) // 非重复依赖任务 ID
		for _, need := range job.Needs {
			if seen[need] {
				continue
			}
			seen[need] = true
			dependency, ok := w.Jobs[need]
			if !ok {
				return fmt.Errorf("workflow %q job %q: depends on unknown job %q", w.Name, id, need)
			}
			dependency.downstream = append(dependency.downstream, id) // 反向添加依赖关系
		}
		job.counter.Store(int32(len(seen)))
	}

	// 创建根模板、加载环境变量
	env := LoadEnv(ctx)
	if w.template == nil {
		w.template = NewTemplate("")
	}
	w.template.Funcs(template.FuncMap{
		"env":  func() map[string]any { return env },
		"jobs": func() map[string]*Job { return w.Jobs },
	})
	err := ExecuteOutputs(w.template, w, w.Env, env)
	if err != nil {
		return fmt.Errorf("workflow %q load env: %w", w.Name, err)
	}
	ctx = context.WithValue(ctx, EnvKey, env)

	// 将所有无依赖的任务放入待执行队列
	w.ready = make(chan string, len(w.Jobs))
	for id, job := range w.Jobs {
		if job.counter.Load() == 0 {
			w.ready <- id
		}
	}

	// 启动并发池
	w.wg.Add(len(w.Jobs))
	for i := 0; i < workerNum; i++ {
		go w.worker(ctx)
	}

	// 等待所有任务结束
	w.wg.Wait()
	close(w.ready)
	err = ctx.Err()
	if err != nil {
		err = fmt.Errorf("workflow %q: %w", w.Name, err)
	}
	for _, job := range w.Jobs {
		if job.err != nil {
			err = wrapError(err, job.err)
		}
	}
	return err
}

// worker 是工作流的工作协程
func (w *Workflow) worker(ctx context.Context) {
	for id := range w.ready {
		// 拿到任务节点并执行
		job := w.Jobs[id]
		success := true
		failure := false
		// 构建任务映射
		needs := make(map[string]*Job, len(job.Needs))
		for _, needID := range job.Needs {
			need := w.Jobs[needID]
			if !need.success {
				success = false
			}
			if need.err != nil {
				failure = true
			}
			needs[needID] = need
		}
		job.template, job.err = w.template.New(w.template.Name() + ".jobs." + id).Clone()
		if job.err == nil {
			job.template.Funcs(template.FuncMap{
				"always":    func() bool { return true },
				"success":   func() bool { return success && ctx.Err() == nil },
				"failure":   func() bool { return failure && ctx.Err() == nil },
				"cancelled": func() bool { return ctx.Err() != nil },
				"needs":     func() map[string]*Job { return needs },
			})
			cond := strings.TrimSpace(job.If)
			if cond == "" {
				cond = "success"
			}
			ok, err := ToValue[bool](job.template, cond, w)
			if err != nil {
				job.err = fmt.Errorf("job %q if %q: %w", job.Name, job.If, err)
			} else if ok {
				// 分情况执行任务，执行工作流或者顺序执行步骤
				if job.Uses != "" {
					if err = job.Use(ctx); err != nil {
						job.err = err
					}
				} else {
					if err = job.Do(ctx); err != nil {
						job.err = err
					}
				}
				if job.err == nil {
					job.success = true
				}
			}
		}
		if job.err != nil {
			job.Outputs["error"] = job.err.Error()
		}
		// 执行成功，通知下游
		for _, downID := range job.downstream {
			downNode := w.Jobs[downID]
			// 依赖全部完成，放入队列
			if downNode.counter.Add(-1) == 0 {
				w.ready <- downID
			}
		}
		w.wg.Done()
	}
}

// Call 调用工作流
func (w *Workflow) Call(ctx context.Context, with map[string]any) (map[string]any, error) {
	// 校验输入
	var err error
	inputs := make(map[string]any, len(w.Inputs))
	for key, input := range w.Inputs {
		inputs[key], err = input.Verify(with[key])
		if err != nil {
			return map[string]any{}, fmt.Errorf("workflow %q input %q: %w", w.Name, key, err)
		}
	}
	w.template = NewTemplate("").Funcs(template.FuncMap{"inputs": func() map[string]any { return inputs }})
	// 执行工作流，因为是由一个协程调用的，所以不让它再展开更多的协程防止指数爆炸
	err = w.Do(ctx, 1)
	if err != nil {
		return map[string]any{}, err
	}
	// 写入输出
	outputs := make(map[string]any, len(w.Outputs))
	for key, output := range w.Outputs {
		outputs[key] = output.Value
	}
	err = ExecuteOutputs(w.template, w, outputs, outputs)
	if err != nil {
		return map[string]any{}, fmt.Errorf(`workflow %q %w`, w.Name, err)
	}
	return outputs, nil
}
