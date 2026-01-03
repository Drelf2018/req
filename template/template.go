package template

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

	"github.com/Drelf2018/req"
)

// Template 模板
type Template struct {
	ID      uint64      `json:"-"           yaml:"-"         gorm:"primaryKey;autoIncrement"`  // 模板标识符
	Author  string      `json:"author"      yaml:"author"    gorm:"index:idx_uses,priority:1"` // 模板作者
	Name    string      `json:"name"        yaml:"name"      gorm:"index:idx_uses,priority:2"` // 模板名称
	Version Version     `json:"version"     yaml:"version"   gorm:"embedded"`                  // 模板版本
	Desc    string      `json:"desc"        yaml:"desc"`                                       // 模板介绍
	Steps   []Step      `json:"steps"       yaml:"steps"     gorm:"foreignkey:TemplateID"`     // 模板步骤
	Env     *OrderedMap `json:"env"         yaml:"env"       gorm:"serializer:json"`           // 模板环境
	Output  *OrderedMap `json:"output"      yaml:"output"    gorm:"serializer:json"`           // 导出变量
}

// StepsString 打印多行子步骤信息
func (t Template) StepsString() string {
	var b strings.Builder
	for _, step := range t.Steps {
		for _, line := range strings.Split(step.String(), "\n") {
			b.WriteString("\n  ")
			b.WriteString(line)
		}
	}
	return b.String()
}

// TemplateName 打印单行模板信息
func (t Template) TemplateName() string {
	return fmt.Sprintf("%s/%s@%s", t.Author, t.Name, t.Version)
}

// String 打印模板全部信息
func (t Template) String() (s string) {
	if t.Env == nil || t.Env.Len() == 0 {
		return fmt.Sprintf("%s/%s@%s%s", t.Author, t.Name, t.Version, t.StepsString())
	}
	return fmt.Sprintf("%s/%s@%s %s%s", t.Author, t.Name, t.Version, t.Env.String(), t.StepsString())
}

// Do 执行子步骤
func (t *Template) Do(ctx context.Context, env *OrderedMap, tmpl *template.Template, data any) error {
	if len(t.Steps) == 0 {
		return nil
	}
	if t.Env == nil {
		t.Env = &OrderedMap{}
	}
	// 创建子步骤的错误集
	var tmplErr TemplateError
	for idx := range t.Steps {
		step := &t.Steps[idx]
		// 创建子模板避免环境变量跨域
		stepTmpl := tmpl.New(fmt.Sprintf("#%d %s", idx, step.StepName()))
		// 初始化子步骤环境变量
		if step.Template.Env == nil {
			step.Template.Env = &OrderedMap{}
		}
		err := Range(stepTmpl, data, step.Template.Env.Clone(), step.Template.Env, step.Template.Env, t.Env, env)
		if err != nil {
			step.Template.Env.Set("ERROR", err)
			tmplErr.Add(stepTmpl.Name(), (*TemplateError)(step.Template.Env))
			continue
		}
		// 执行子步骤
		err = step.Do(ctx, env, stepTmpl, data)
		if err != nil {
			step.Template.Env.Set("ERROR", err)
			tmplErr.Add(stepTmpl.Name(), (*TemplateError)(step.Template.Env))
		}
	}
	return tmplErr.Unwrap()
}

// Step 步骤
type Step struct {
	Template   `yaml:",inline"`
	TemplateID uint64         `json:"-"       yaml:"-"`                             // 模板外键
	Uses       string         `json:"uses"    yaml:"uses"`                          // 使用模板
	Method     string         `json:"method"  yaml:"method"`                        // 请求方法
	URL        string         `json:"url"     yaml:"url"`                           // 请求地址
	Body       any            `json:"body"    yaml:"body"   gorm:"serializer:json"` // 请求内容
	Cookie     any            `json:"cookie"  yaml:"cookie" gorm:"serializer:json"` // 请求 Cookie
	Header     map[string]any `json:"header"  yaml:"header" gorm:"serializer:json"` // 请求头部
}

// StepName 打印单行步骤信息
func (s Step) StepName() string {
	if s.URL != "" {
		return s.Method + " " + s.URL
	} else if s.Uses != "" {
		return s.Uses
	}
	return s.Template.TemplateName()
}

// String 打印多行步骤信息，包含子步骤
func (s Step) String() string {
	if s.URL != "" {
		return strings.Join([]string{s.Method, s.URL, s.Template.Env.String(), s.Template.StepsString()}, " ")
	} else if s.Uses != "" {
		return strings.Join([]string{s.Uses, s.Template.Env.String(), s.Template.StepsString()}, " ")
	}
	return s.Template.String()
}

// Request 用步骤发送请求
func (s Step) Request(ctx context.Context, env *OrderedMap, tmpl *template.Template, data any) (err error) {
	// 初始化请求地址
	s.URL, err = ToString(tmpl, s.URL, data, s.Template.Env, env)
	if err != nil {
		return
	}
	// 有默认请求任务池则用请求池，否则直接发送请求
	request := &Request{Tmpl: tmpl, Data: data, Step: &s, Getter: []*OrderedMap{s.Template.Env, env}}
	var r []byte
	if req.DefaultPool != nil {
		r, err = req.DefaultPool.NewTaskWithContext(ctx, request).Content()
	} else {
		r, err = req.ContentWithContext(ctx, request)
	}
	if err != nil {
		return
	}
	// 将步骤运行结果输出到全局环境变量
	if s.Template.Output == nil || s.Template.Output.Len() == 0 {
		return
	}
	// 请求成功，将响应体写进环境变量
	result := &OrderedMap{}
	result.Set("content", r)
	result.Set("text", string(r))
	// 尝试反序列化 JSON
	var i any
	result.Set("error", json.Unmarshal(r, &i))
	result.Set("json", i)
	return Range(tmpl, data, s.Template.Output, env, result, s.Template.Env, env)
}

// Load 用步骤加载子模板
func (s Step) Load(ctx context.Context, env *OrderedMap, tmpl *template.Template, data any) (err error) {
	// 使用默认加载器加载子模板
	loadedTmpl, err := Load(s.Uses)
	if err != nil {
		return
	}
	// 用步骤变量覆盖子模板变量
	err = Range(tmpl, data, s.Template.Env, loadedTmpl.Env, env)
	if err != nil {
		return
	}
	// 运行子模板
	loadedEnv := &OrderedMap{}
	err = Step{Template: loadedTmpl}.Do(ctx, loadedEnv, tmpl, data)
	if err != nil {
		return
	}
	// 将子模板运行结果输出到全局环境变量
	if s.Template.Output == nil || s.Template.Output.Len() == 0 {
		return
	}
	return Range(tmpl, data, s.Template.Output, env, loadedEnv)
}

// Do 执行步骤，如果步骤有 URL 则发送请求，如果有 Uses 则加载并执行子模板，否则视为起始模板
func (s Step) Do(ctx context.Context, env *OrderedMap, tmpl *template.Template, data any) (err error) {
	if s.URL != "" {
		err = s.Request(ctx, env, tmpl, data)
	} else if s.Uses != "" {
		err = s.Load(ctx, env, tmpl, data)
	} else {
		env.Update(s.Template.Env)
		if s.Template.Output != nil && s.Template.Output.Len() != 0 {
			err = Range(tmpl, data, s.Template.Output, env, s.Template.Env, s.Template.Output, env)
		}
	}
	if err != nil {
		return
	}
	// 执行自身模板子步骤
	return s.Template.Do(ctx, env, tmpl, data)
}
