# 123pan-goctl

## 项目简介
123pan-goctl 是一个基于 Go 语言开发的 123云盘非官方命令行客户端，旨在提供便捷的 API 接口封装，使开发者能够轻松地与 123云盘进行交互，如文件上传、列表查询、重命名等操作。

## 使用说明

### 安装
使用go install
```bash
go install github.com/no-mole/123pan-goctl
```
 [直接下载二进制](https://github.com/no-mole/123pan-goctl/releases)。

### 配置 Config
使用本项目前，需要配置 `$HOME/.123pan.yaml` 以提供必要的 API 认证信息：
```yaml
client_id: xxxx
client_secret: xxxx
```

## 项目目标
本项目旨在实现以下 API 接口，提供完整的 123云盘操作能力：

- **文件操作**
    - [x] `upload` - 上传文件
    - [ ] `list` - 获取文件列表
    - [ ] `mv` - 移动文件
    - [ ] `rename` - 重命名文件
    - [ ] `info` - 获取文件信息

- **用户操作**
    - [x] `info` - 获取用户信息

更多功能请参考 [官方文档](https://123yunpan.yuque.com/org-wiki-123yunpan-muaork/cr6ced)。