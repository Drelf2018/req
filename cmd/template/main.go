package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Drelf2018/req/template"
	"gopkg.in/yaml.v3"
)

var ErrEmptyTemplatePath = errors.New("req/cmd/template: empty template path")

func init() {
	template.DefaultLoader = template.FileLoader{}
}

func close(c io.Closer) {
	if err := c.Close(); err != nil {
		fmt.Println(err)
	}
}

func main() {
	if len(os.Args) < 2 {
		panic(ErrEmptyTemplatePath)
	}
	tmpl, err := template.Load(os.Args[1])
	if err != nil {
		panic(err)
	}
	// 解析数据
	if tmpl.Env == nil {
		tmpl.Env = &template.OrderedMap{}
	}
	for _, expr := range os.Args[2:] {
		k, v, ok := strings.Cut(expr, "=")
		if !ok {
			continue
		}
		tmpl.Env.Set(strings.TrimPrefix(k, "--"), v)
	}
	input := tmpl.Env.Map()
	// 运行模板，添加错误信息
	env := &template.OrderedMap{}
	err = template.Step{Template: tmpl}.Do(context.Background(), env, template.NewTemplate("root"), nil)
	if err != nil {
		fmt.Println(err)
		env.Set("[ERROR]", err)
	}
	// 过滤输入
	env.Clone().Iterate(func(key string, value any) bool {
		if input[key] == value {
			env.Del(key)
		}
		return true
	})
	// 把结果写入文件
	folder := strings.TrimSuffix(os.Args[1], filepath.Ext(os.Args[1]))
	err = os.MkdirAll(folder, os.ModePerm)
	if err != nil {
		panic(err)
	}
	file, err := os.OpenFile(fmt.Sprintf("%s/%s.yml", folder, time.Now().Format("2006-01-02-15-04-05")), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.ModePerm)
	if err != nil {
		panic(err)
	}
	defer close(file)
	encoder := yaml.NewEncoder(file)
	defer close(encoder)
	encoder.SetIndent(2)
	err = encoder.Encode(env)
	if err != nil {
		panic(err)
	}
}
