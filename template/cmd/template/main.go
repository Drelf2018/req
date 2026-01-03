package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Drelf2018/req/template"
)

var ErrEmptyTemplatePath = errors.New("req/template/cmd/template: empty template path")

func init() {
	template.DefaultLoader = template.FileLoader{}
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
	env.Set("ERROR", template.Step{Template: tmpl}.Do(context.Background(), env, template.NewTemplate("root"), nil))
	// 过滤输入
	for key, value := range env.Clone().Iterate {
		if input[key] == value {
			env.Del(key)
		}
	}
	// 把结果写入文件
	b, err := json.Marshal(env)
	if err != nil {
		panic(err)
	}
	folder := strings.TrimSuffix(os.Args[1], filepath.Ext(os.Args[1]))
	err = os.MkdirAll(folder, os.ModePerm)
	if err != nil {
		panic(err)
	}
	err = os.WriteFile(fmt.Sprintf("%s/%s.json", folder, time.Now().Format("2006-01-02-15-04-05")), b, os.ModePerm)
	if err != nil {
		panic(err)
	}
}
