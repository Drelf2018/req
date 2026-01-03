package template

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
)

// Loader 用于加载模板
type Loader interface {
	Load(uses string, tmpl *Template) error
}

// FileLoader 文件加载器，可以用 JSON 和 YAML 文件加载模板
type FileLoader struct{}

func (FileLoader) Load(uses string, tmpl *Template) error {
	b, err := os.ReadFile(uses)
	if err != nil {
		return err
	}
	ext := filepath.Ext(uses)
	switch strings.ToLower(ext) {
	case ".json":
		err = json.Unmarshal(b, tmpl)
	case ".yml", ".yaml":
		err = yaml.Unmarshal(b, tmpl)
	default:
		err = fmt.Errorf("req/template: invalid file type: %q", ext)
	}
	return err
}

var _ Loader = FileLoader{}

// DatabaseLoader 数据库加载器，可以查询数据库 steps 表加载模板
type DatabaseLoader struct {
	*gorm.DB
}

func (d DatabaseLoader) Load(uses string, tmpl *Template) error {
	author, uses, ok := strings.Cut(uses, "/")
	if !ok {
		return fmt.Errorf("req/template: invalid template uses: expected \"author/namespace@vX.Y.Z\", got: %q", uses)
	}
	namespace, version, ok := strings.Cut(uses, "@")
	if !ok {
		return fmt.Errorf("req/template: invalid template uses: expected \"author/namespace@vX.Y.Z\", got: %q", uses)
	}
	var ver Version
	err := ver.UnmarshalText([]byte(version))
	if err != nil {
		return err
	}
	result := d.DB.Model(&Step{}).Preload("Steps").Limit(1).Scopes(ver.Match).Find(tmpl, `author = ? AND namespace = ?`, author, namespace)
	if result.Error != nil {
		return fmt.Errorf("req/template: load template from database error: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("req/template: template uses not found: %q", uses)
	}
	return nil
}

var _ Loader = DatabaseLoader{}

// DefaultLoader 默认加载器
var DefaultLoader Loader

// ErrNilDefaultLoader 默认加载器为空错误
var ErrNilDefaultLoader = errors.New("req/template.Load: DefaultLoader is nil")

// ErrTemplateCycle 模板引用循环
var ErrTemplateCycle = errors.New("req/template.Load: template cycle not allowed")

// Load 使用默认加载器加载模板，会检查是否存在引用循环
func Load(uses string) (Template, error) {
	if DefaultLoader == nil {
		return Template{}, ErrNilDefaultLoader
	}
	// 判断是否有循环引用
	path := make([]string, 0)
	visited := make(map[string]bool)
	visiting := make(map[string]bool)
	templates := make(map[string]Template)
	// 定义深度优先遍历
	var dfs func(node string) error
	dfs = func(node string) error {
		if node == "" {
			return nil
		}
		// 已经验证过无环
		if visited[node] {
			return nil
		}
		// 未验证过，添加至路径
		path = append(path, node)
		// 已经在路径上
		if visiting[node] {
			return ErrTemplateCycle
		}
		visiting[node] = true
		// 获取其子引用
		var nodeTmpl Template
		err := DefaultLoader.Load(node, &nodeTmpl)
		if err != nil {
			return fmt.Errorf("req/template.Load: failed to load %q: %w", node, err)
		}
		templates[node] = nodeTmpl
		// 开始遍历
		for _, step := range templates[node].Steps {
			err := dfs(step.Uses)
			if err != nil {
				return err
			}
		}
		// 无环
		path = path[:len(path)-1]
		visiting[node] = false
		visited[node] = true
		return nil
	}
	// 开始验证
	if err := dfs(uses); err != nil {
		return Template{}, &CycleError{err, path}
	}
	return templates[uses], nil
}

// CycleError 循环引用错误
type CycleError struct {
	Err   error    // 包裹错误
	Cycle []string // 循环链
}

func (e *CycleError) Error() string {
	if len(e.Cycle) == 0 {
		return e.Err.Error()
	}
	var b strings.Builder
	b.WriteString(" (")
	for i, uses := range e.Cycle {
		b.WriteString(uses)
		if i != len(e.Cycle)-1 {
			b.WriteString(" → ")
		}
	}
	b.WriteByte(')')
	return fmt.Sprintf("%s%s", e.Err, b.String())
}
