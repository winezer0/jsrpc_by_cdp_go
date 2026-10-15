# Chrome CDP 浏览器操作说明

## 项目目标

本项目用于通过 Chrome DevTools Protocol 连接浏览器、打开目标页面、定位目标脚本、设置断点，并在断点上下文中提升签名对象、注册 `sign` 动作和验证签名函数。

Python版 jsrpc_by_cdp_python[https://github.com/winezer0/jsrpc_by_cdp_python]

## 当前流程

1. 检测 `http://127.0.0.1:<port>/json/version` 是否可访问。
2. 若不可访问，自动启动 Chrome 并开启 CDP 调试端口。
3. 连接页面级 WebSocket。
4. 启用 `Page`、`Runtime`、`Debugger`、`Network` 域。
5. 打开目标页面并等待加载完成。
6. 收集页面脚本 URL 和 `Debugger.scriptParsed` 事件。
7. 优先使用 `Debugger.getScriptSource` 获取脚本源码。
8. 当未拿到 `scriptId` 时，自动回退到 `httpx` 下载脚本源码。
9. 依据目标代码片段定位断点位置并设置精确断点。
10. 刷新页面触发暂停。
11. 在暂停上下文中执行提升脚本，得到可用的签名函数。
12. 注入 `js_env.js`。
13. 读取 `reg_action.js` 注册 `sign` 动作。
14. 读取 `reg_action_verify.js` 校验动作是否注册成功。
15. 读取 `promote_func_verify.js` 验证签名函数是否可用。
16. 恢复浏览器执行并保持连接一段时间。

## 脚本源码获取策略

- 优先使用 `Debugger.scriptParsed` + `Debugger.getScriptSource`
- 事件采集改为事件缓存模式，减少 WebSocket 消息抢读问题
- 当未拿到 `scriptId` 时，自动刷新页面重试
- 若仍无法获取，则回退到 `httpx` 下载脚本源码

## 断点与签名流程

1. 先根据目标脚本和目标代码片段定位精确断点。
2. 页面刷新后触发断点暂停。
3. 在暂停上下文中执行 `promote_func.js`。
4. 注入 `js_env.js`。
5. 读取 `reg_action_context.js` 检查注册前置条件。
6. 读取 `reg_action.js` 注册 `sign` 动作。
7. 读取 `reg_action_verify.js` 检查动作注册结果。
8. 读取 `promote_func_verify.js` 验证签名函数是否可用。

## 调试输出

脚本会输出以下信息，方便排查问题：

- CDP 服务是否已就绪
- Chrome 是否自动启动成功
- 页面中已有的 JS 列表
- `Debugger.scriptParsed` 捕获到的 JS 列表
- 当前使用的是 CDP 源码还是 `httpx` 源码
- 断点位置
- `sign` 动作注册前置条件
- `sign` 动作注册结果
- `sign` 动作校验结果
- 签名函数验证结果

## 手动测试分析是否已经成功链接jsrpc客户端
js_env.js 中在客户端连接服务端时注册了一个 hello 接口 
```
//写一个传入字符串，返回base64值的接口(调用内置函数btoa)
demo.regAction("hello", function (resolve,param) {
    console.log(param);
    var base666 = btoa(param)
    resolve(base666);
})
```
//本机任意浏览器访问接口，获得js端的返回值
```
http://127.0.0.1:12080/go?group=zzz&action=hello&param=123456
```
//客户端未注册会返回:
```
{"clientId":"xxx","data":"action没找到","group":"zzz","status":200}
```
//客户端已经注册会返回:
```
{"clientId":"xxx","data":"MTIzNDU2","group":"zzz","status":200}
```


## 常见问题

### 1. `Debugger.scriptParsed` 捕获数量为 0

可能原因：

- 页面脚本解析事件发生太早
- 事件尚未进入缓存
- 目标脚本尚未加载完成

处理方式：

- 保持当前事件缓存模式
- 必要时回退到 `httpx`

### 2. `sign` 动作注册失败

可能原因：

- `demo` 未注入成功
- `demo.regAction` 不存在
- `reg_action_context.js` 的前置条件不满足
- `reg_action.js` 的注册逻辑与目标页面不匹配

### 3. Chrome 未能自动启动

可能原因：

- Chrome 路径配置错误
- 用户数据目录不可写
- 端口被占用
