### 是什么

这是一个利用 DeepSeek 评论指定用户最新一条微博的模板。

### 怎么用

```
> go install github.com/Drelf2018/req/cmd/template@latest
> template automatic_comment.yml --uid="" --key="" --weibo=""
```

`uid` 是要评论的博主的 `UID`

`key` 是用来调用 `DeepSeek` 的 `api_key`

`weibo` 为发送评论的微博账号的 `Cookie`
