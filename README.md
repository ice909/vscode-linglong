# vscode 玲珑插件

编写玲珑 linglong,yaml 文件

## 代码提示和校验

使用 schemas 实现 yaml 文件的代码提示和必填字段校验

![image](./image.png)

## 依赖补全

在制作玲珑包时，如果 base 和 runtime 中缺少构建应用所需依赖，需要自己添加 deb 包，deb 包的依赖繁多，软件更新后，下载地址又会失效，插件提供了一种临时方案。

```yaml
build: |
  ...
sources:
  - kind: git
    url: https://github.com/linuxdeepin/dde-calendar
    commit: 5.13.1
  # linglong:gen_deb_source sources arm64 https://pools.uniontech.com/deepin-beige beige main community
  # linglong:gen_deb_source install libical-dev, wget
```

将 sources 字段放在 linglong.yaml 的最后，然后在 sources 最后添加以上注释(将 libical-dev, wget 替换称你自己想安装的软件包)

第一个注释是用于设置软件源，只能写一次，第二个注释是用于安装依赖，可以写多次，例如`libical-dev, wget`可以分开写：

```
# linglong:gen_deb_source install libical-dev
# linglong:gen_deb_source install wget
```

添加这两行注释后，在 vscode 中按`Ctrl+Shift+P` 搜索 `linglong: Gen deb sources`, 等待一段时间后，插件会自动将填写的依赖和其依赖树全部添加到当前 linglong.yaml 文件末尾，之后依赖有更新，可重复执行
`linglong: Gen deb sources`命令来更新软件包

## FAQ

为什么按 `Ctrl+Shift+P` 无法搜索到相关命令

首先要确保当前 vscode 安装了本插件，其次要确定当前编辑器正打开着 linglong.yaml 文件
