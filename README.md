# Azure Resource Retry 插件

这个 CLIProxyAPI 插件用于 Codex Responses 请求，基于 CLIProxyAPI v8.0.70 的插件接口构建（ABI 1、协议版本 10）。请求先按原始历史发送；当上游返回以下 400 错误时，插件才会清理历史 item 标识并重试一次：

```text
The requested item was created under a different OpenAI resource.
```

同样识别 `different Azure OpenAI resource` 等在 `OpenAI resource` 前带资源限定词的消息。只有 HTTP 状态码为 400 且错误消息匹配时才触发。

清理内容包括历史输入 item 的 `id`、`encrypted*` 字段、`item_reference`，顶层 `previous_response_id`，以及 `include` 中的 `reasoning.encrypted_content`。清理仅作用于 `input` 数组第一层 item，保留 `call_id`、`tools`、`arguments` 和工具输出中的嵌套业务字段。其他错误不会触发清理；重试只执行一次。

插件使用原始请求正文委托宿主执行，工具 schema 和用户 payload 配置由目标执行器按原生规则处理。

插件同时支持非流式和 SSE 流式 Responses 请求。流式请求在上游流打开后只转发内容；流中断不会重新执行请求。

客户端需发送完整历史消息和工具调用内容；只有 `previous_response_id` 或 `item_reference` 的请求无法从引用中还原历史正文。

## 通过第三方插件源安装

在 CLIProxyAPI 管理面板的插件商店中添加第三方源：

```text
https://raw.githubusercontent.com/XAGU/cpa-plugin-azure-resource-retry/main/registry.json
```

也可以将以下配置合并到 `config.yaml`，然后在插件商店安装并启用 `azure-resource-retry`：

```yaml
plugins:
  enabled: true
  dir: "plugins"
  store-sources:
    - "https://raw.githubusercontent.com/XAGU/cpa-plugin-azure-resource-retry/main/registry.json"
  configs:
    azure-resource-retry:
      enabled: true
      priority: 100
```

发布资产支持以下平台：

| 系统 | 架构 | 动态库 |
| --- | --- | --- |
| Linux | amd64、arm64 | `azure-resource-retry.so` |
| macOS | amd64、arm64 | `azure-resource-retry.dylib` |
| Windows | amd64、arm64 | `azure-resource-retry.dll` |
| FreeBSD | amd64 | `azure-resource-retry.so` |

Linux 资产依赖 glibc，使用 Debian Bookworm 构建，适用于 CLIProxyAPI 的 Debian Docker 镜像。FreeBSD 资产使用 14.4 系统库构建。

Docker Compose 部署时，在现有服务中添加目录挂载：

```yaml
services:
  cli-proxy-api:
    volumes:
      - ./plugins:/CLIProxyAPI/plugins
```

插件只接管 `openai-response` 和 `codex` 的 Responses 请求。

## 从源码构建

需要 Go 1.26.10 或更新版本、目标平台的 C 编译器，并启用 CGO。默认构建当前平台：

```sh
sh ./build.sh
```

Linux amd64 构建与安装：

```sh
GOOS=linux GOARCH=amd64 sh ./build.sh
mkdir -p plugins
cp dist/azure-resource-retry.so plugins/azure-resource-retry.so
```

动态库文件名去掉扩展名后就是插件 ID，需保持为 `azure-resource-retry`。跨平台构建时需自行配置匹配目标平台的 `CC` 和系统库。

## GitHub 自动构建与发布

推送 `main` 分支、创建 Pull Request 或手动运行工作流会执行测试、静态检查和全部 7 个目标的构建。

推送 `v0.1.1` 形式的标签会自动创建 GitHub Release，并上传：

```text
azure-resource-retry_0.1.1_linux_amd64.zip
azure-resource-retry_0.1.1_linux_arm64.zip
azure-resource-retry_0.1.1_darwin_amd64.zip
azure-resource-retry_0.1.1_darwin_arm64.zip
azure-resource-retry_0.1.1_windows_amd64.zip
azure-resource-retry_0.1.1_windows_arm64.zip
azure-resource-retry_0.1.1_freebsd_amd64.zip
checksums.txt
```

ZIP 根目录包含对应平台的动态库和 `LICENSE`。`checksums.txt` 记录所有 ZIP 的 SHA-256 校验值。动态库注册版本与标签中的数字版本一致。

```sh
git tag v0.1.1
git push origin v0.1.1
```

插件商店从仓库最新 Release 获取可安装版本，发布新版本时无需修改 `registry.json`。

## 验证

- `go test ./... ./.github/scripts`、`go vet ./... ./.github/scripts`。
- 在 Windows 上使用 CLIProxyAPI v8.0.70、插件动态库和模拟 Codex 上游验证正常请求、非流式重试、流式重试、无关错误和单次重试上限；使用 Codex User-Agent 验证保留工具 schema 与用户 payload 配置。
- Linux amd64 产物已交叉编译，并检查 ELF 架构、依赖库和 ABI 导出符号；未在 Linux/Docker 或真实 Azure 资源中运行验证。

## 许可证

[MIT](LICENSE)
