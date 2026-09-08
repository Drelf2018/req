<div align="center">

# template

_✨ 通过 YAML 编排请求 ✨_

</div>

## 什么是 `template`

本模块是一个工作流引擎，用于编排一系列 HTTP 请求。你可以通过编写 `YAML` 文件来描述工作流，其中包含多个任务 `Job` 与步骤 `Step` ，还可以把常用的步骤封装成可复用的动作 `Action` 。引擎负责解析、检测依赖环、并发调度并执行它们。

```yaml
name: 简单工作流
jobs:
  request:
    name: 发起请求
    steps:
      - id: httpbin
        name: 请求 httpbin
        curl: https://httpbin.org/get
        outputs:
          $origin: 'gjson "origin" text'
```

简单来说，工作流拥有一个任务映射，每个任务拥有自己的步骤集。任务可以调用其他工作流，步骤可以调用动作。

## 快速开始

```go
package main

import (
	"context"

	"github.com/Drelf2018/req/template"
)

func main() {
	w, err := template.UnmarshalWorkflow("workflow.yml")
	if err != nil {
		panic(err)
	}
	// Do 会在执行前自动检测依赖环，并以 4 个并发协程调度任务
	err = w.Do(context.Background(), 4)
	if err != nil {
		panic(err)
	}
}
```

## 步骤 `Step`

步骤是工作流的最小执行单元，包含以下字段：

| 字段 | 说明 |
| --- | --- |
| `id` | 步骤标识符，供其他步骤通过 `steps.<id>.Outputs.<key>` 引用其输出 |
| `if` | 执行条件，支持 `success` `failure` `cancelled` `always` 等类似 Github Action 的控制符，默认为 `success` |
| `name` | 步骤名称 |
| `curl` | 请求命令，支持 `-X` `-G` `-d` `-H` `-b` `-u` `-e` `-A` 等常用参数 |
| `uses` | 引用的动作，与 `curl` 互斥 |
| `env` | 环境变量，可用于填充 `curl` 与其他模板 |
| `with` | 输入参数 |
| `outputs` | 输出参数 |
| `timeout` | 超时秒数，默认为 60 秒 |

### 请求命令 `curl` 

直接把从浏览器或抓包工具复制的 `curl` 命令粘贴进步骤即可：

```yaml
name: 发送请求
jobs:
  send:
    name: 发送
    steps:
      - name: 发送表单
        curl: https://httpbin.org/post -d "name=Nana7mi" -H "Content-Type:application/x-www-form-urlencoded"
```

默认只接受 `200 OK` 响应，其余状态码会报错。

如果你只想借用 `curl` 命令的解析能力而不使用工作流引擎，可以直接调用以下函数获取 `*http.Request` 对象：

```go
req, err := template.CURLToRequest(ctx, `curl https://httpbin.org/get`)
req, err = template.ArgsToRequest(ctx, "-G", "https://httpbin.org/get", "-d", "a=1")
```

### 引用动作 `uses` 

`uses` 用于引用一个可复用动作 `Action` ，详见下文。它和 `curl` 只能二选一。

### 环境变量 `env`

环境变量通过 `env` 字段声明，在模板中以 `env.<key>` 访问：

```yaml
name: 环境变量示例
env:
  base: https://httpbin.org
jobs:
  request:
    name: 请求
    env:
      path: /get
    steps:
      - name: 请求
        env:
          query: ?a=1
        curl: '{{ env.base }}{{ env.path }}{{ env.query }}'
```

环境变量会从工作流、任务逐步继承至步骤，内层可见外层。动作和子工作流的环境变量与外部**隔离**，只能通过 `with` 传入。

### 输出参数 `outputs`

步骤的 `outputs` 用于从本次响应中提取结果，它的写法有两种：

```yaml
name: 输出示例
jobs:
  request:
    name: 请求
    steps:
      - name: 请求 httpbin
        curl: https://httpbin.org/get
        outputs:
          origin1: '{{ gjson "origin" text }}' # 普通键：对于非字符串值直接写入输出，否则通过模板写入要输出的值的字符串形式
          $origin2: result.origin # 特殊键：会通过模板在去掉 $ 的键名写入要输出的值的原始类型
```

模板中可以使用 `response` `content` `result` `text` 四个函数获取本次请求的响应、响应体内容、响应体反序列化对象和响应体字符串。因此，从 `text` 中用 `gjson` 取出 `"origin"` 等价于直接获取 `result.origin` 。特殊键的结果会**覆盖**同名普通键的值。

## 步骤集 `Steps`

步骤集是一组按顺序执行的步骤，任务与动作中的 `steps` 都是步骤集。步骤按声明顺序依次执行，某个步骤失败不会中断后续步骤，所有错误会在最后汇总返回。步骤中还可以使用快捷函数 `outputs` 读取**最后一个成功步骤**的输出。

同一步骤集共用一个 `*http.Client` 客户端：前面的步骤登录后，后续步骤会自动携带 Cookie 发送请求，不同任务、不同动作之间的会话互不影响。客户端配置继承自全局 `http.DefaultClient` 客户端。

### 执行条件 `if`

每个步骤的 `if` 控制符决定它是否执行，引擎会用模板计算表达式的真假值。

```yaml
name: 条件示例
jobs:
  request:
    name: 请求
    steps:
      - id: first
        name: 获取用户
        curl: https://httpbin.org/get
        outputs:
          $origin: 'gjson "origin" text'

      - name: 使用上一步结果
        if: 'not (eq outputs.origin "")'
        curl: https://httpbin.org/anything -d "origin={{ outputs.origin }}"
```

## 动作 `Action`

动作是步骤的集合，可以被 `uses` 引用，包含以下字段：

| 字段          | 说明                                   |
| ------------- | -------------------------------------- |
| `name`        | 动作名称                       |
| `description` | 动作描述                               |
| `author`      | 动作作者                               |
| `inputs`      | 动作输入                               |
| `outputs`     | 动作输出                               |
| `steps`       | 动作步骤                 |

它可以声明输入与输出：

```yaml
# get_ip.yml
name: 获取 IP 信息
description: 通过 httpbin 获取请求来源 IP
author: Drelf2018
inputs:
  verbose:
    description: 是否输出详细信息
    required: false
    type: bool
    default: false
outputs:
  $origin:
    description: 请求来源 IP
    value: steps.httpbin.Outputs.origin
steps:
  - id: httpbin
    name: 请求 httpbin
    curl: https://httpbin.org/get
    outputs:
      $origin: result.origin
```

在其他文件的步骤中调用可复用动作：

```yaml
name: 调用动作示例
jobs:
  get:
    name: 获取 IP
    steps:
      - uses: get_ip.yml
        with:
          verbose: true
```

### 输入参数 `with`

在使用 `uses` 调用动作之后，使用 `with` 向其传递参数。

### 输入 `inputs`

`inputs` 中的每个输入都会经过校验：

- 没有输入时，如果是必填项则报错，否则检查是否提供默认值，如果没有提供默认值，则返回空字符串。
- 如果提供了默认值，则设置输入为默认值，并与有输入的情况一同匹配类型是否正确。

### 输出 `outputs`

动作的 `outputs` 是声明式的，每个输出包含 `description` 与 `value`，`value` 是模板表达式，通常引用步骤的输出：

```yaml
name: 部署动作
description: 部署新版本
outputs:
  $version:
    description: 版本号
    value: steps.build.Outputs.version
  message:
    description: 部署信息
    value: '部署了版本 {{ steps.build.Outputs.version }}'
steps:
  - id: build
    name: 构建
    curl: https://httpbin.org/get
    outputs:
      $version: result.origin
```

特殊输出键的用法与步骤输出参数一致，会用模板将 `value` 的值保存在输出中。

## 任务 `Job`

任务是工作流的执行单元，包含以下字段：

| 字段 | 说明 |
| --- | --- |
| `if` | 执行条件，同步骤，默认为 `success` |
| `name` | 任务名称 |
| `needs` | 依赖任务，可以是单个字符串或字符串列表 |
| `uses` | 引用的子工作流，与 `steps` 互斥 |
| `env` `with` `outputs` | 环境变量、输入参数、输出参数，同步骤 |
| `timeout` | 超时秒数，默认为步骤数 × 60 秒 |
| `steps` | 任务步骤集 |

任务的 `outputs` 写法与步骤相同，只是模板中多了 `steps.<id>.Outputs.<key>` 可以引用本任务内步骤的输出。任务执行失败时，错误信息会写入 `outputs.error` 键，方便在后续任务或序列化结果中读取失败原因。

### 依赖任务 `needs`

`needs` 声明本任务依赖的其他任务，可以是单个字符串或字符串列表。任务会在其依赖的任务全部完成后才会开始执行。上游任务失败时，下游默认会被跳过。任务之间互相依赖会形成依赖环，`Do` 会在执行前检测并报错。

### 引用子工作流 `uses`

任务也可以通过 `uses` 引用另一个工作流文件，它与 `steps` 互斥。被引用的工作流可以像动作一样声明输入与输出：

```yaml
# deploy_flow.yml
name: 部署工作流
inputs:
  target:
    description: 部署目标
    required: true
    type: string
outputs:
  $version:
    description: 部署版本
    value: jobs.build.Outputs.version
jobs:
  build:
    name: 构建
    steps:
      - id: build
        name: 构建
        curl: https://httpbin.org/get
        outputs:
          $origin: result.origin
    outputs:
      $version: steps.build.Outputs.origin
```

```yaml
name: 部署
jobs:
  deploy:
    name: 部署
    uses: deploy_flow.yml
    with:
      target: production
```

## 工作流 `Workflow`

工作流由多个任务组成，包含以下字段：

| 字段 | 说明 |
| --- | --- |
| `name` | 工作流名称 |
| `inputs` | 工作流输入，在 `Call` 时校验 |
| `outputs` | 工作流输出，声明式，同动作 |
| `env` | 环境变量 |
| `jobs` | 任务映射 |

任务之间可以声明依赖关系：

```yaml
name: 并发工作流
jobs:
  build:
    name: 构建
    steps:
      - name: 请求 httpbin
        curl: https://httpbin.org/get

  deploy:
    name: 部署
    needs: build
    steps:
      - name: 请求 httpbin
        curl: https://httpbin.org/post -d "deployed=true"
```

### 并发调度 `Do`

`Do` 的第二个参数是并发协程数。没有 `needs` 依赖的任务会被并发执行，其他任务在其 `needs` 引用的任务全部完成后才会开始执行。任务的 `if` 同样支持 `success` `failure` `cancelled` `always` 控制符，默认为 `success` 。上游任务失败时，下游默认会被跳过。上下文被取消后，声明了 `cancelled` 或 `always` 的任务仍然会被执行（用于上传错误日志等收尾工作），此时任务内部会改用不受取消影响的上下文用于发送请求。

在 Go 代码中也可以直接调用带输入校验的 `Call` 方法，嵌套调用时会以单协程执行防止协程爆炸：

```go
outputs, err := w.Call(ctx, map[string]any{"target": "production"})
```

执行结束后，还可以通过 `Result` 方法查看任务的执行结果（`success` `failure` `skipped`）：

```go
w.Jobs["deploy"].Result()
```

## 模板函数

在模板中可以使用以下内置函数：

| 函数 | 说明 |
| --- | --- |
| `int` `str` | 字符串与整数互转 |
| `reg` | 正则查找 |
| `type` | 获取值的类型名 |
| `json` | 序列化为 JSON |
| `gjson` | 从 JSON 中提取值 |
| `base64encode` `base64decode` | Base64 编解码 |
| `plaintext` | 提取去除 HTML 标签的纯文本 |

## 依赖环检测

`Do` 在执行前会自动检测两类环，发现环会直接返回错误而不是挂死：

- `needs` 依赖环（任务之间互相依赖）
- `uses` 引用环（动作/工作流之间互相引用，包括跨文件嵌套）

检测结果通过上下文标记传递，嵌套调用时整条链只检测一次。

## 自定义加载

`uses` 引用的节点通过以下两个函数变量加载，默认读取本地文件：

```go
// UnmarshalAction 反序列化可复用动作
var UnmarshalAction = func(name string) (*Action, error) {
	// 默认从文件读取，可以替换为网络请求、数据库查询等
}

// UnmarshalWorkflow 反序列化工作流
var UnmarshalWorkflow = func(name string) (*Workflow, error) {
	// 同上
}
```

默认反序列化实现从文件读取，因此环检测的 `NormalizeUses` 默认按文件路径归一化（合并同一文件的不同写法，如 `./a.yml` 与 `a.yml`）。若替换了加载函数，请一并替换它以匹配新的标识语义。
