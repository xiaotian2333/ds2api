# 透明转发改造方案

## 1. internal/prompt/messages.go

### 常量修改（L19-28）

```go
const (
    beginSentenceMarker        = ""
    systemMarker               = ""
    userMarker                 = "用户："
    assistantMarker            = "助手："
    toolMarker                 = "工具："
    endSentenceMarker          = ""
    endToolResultsMarker       = ""
    endInstructionsMarker      = ""
)
```

### 函数 MessagesPrepareWithThinking 修改

**移除 L57：** `parts = append(parts, beginSentenceMarker)` — 已为空，去掉

**case "system" 修改（L69-71）：**
```go
case "system":
    if text := strings.TrimSpace(m.Text); text != "" {
        parts = append(parts, text) // 直接追加内容，无标记
    }
```

**case "tool" 修改（L64-67）：**
```go
case "tool":
    parts = append(parts, "\n【系统提示：此环境不可调用工具，调用已被拦截】\n")
```

## 2. internal/prompt/messages_test.go

| 行号 | 旧值 | 新值 |
|------|------|------|
| L35 | `!strings.HasPrefix(got, beginSentenceMarker+systemMarker+"System rule")` | `!strings.Contains(got, "System rule")` |
| L38 | 移除整个 `if !strings.Contains(got, "<｜end▁of▁instructions｜>")` 块 | 删除 |
| L41 | `"<｜User｜>Question"` | `"用户：Question"` |
| L44 | `"<｜Assistant｜>Answer<｜end▁of▁sentence｜>"` | `"助手：Answer"` |
| L68 | `"<｜Assistant｜>"` | `"助手："` |

## 3. internal/util/messages_test.go

| 行号 | 旧值 | 新值 |
|------|------|------|
| L16 | `"<｜begin▁of▁sentence｜><｜User｜>"` | `"用户："` |
| L19 | `"<｜Assistant｜>"` | `"助手："` |
| L33 | `"<｜User｜>Hi"` | `"用户：Hi"` |
| L36 | 移除整个 `if !contains(got, "<｜begin▁of▁sentence｜>")` 块 | 删除 |
| L39 | `"<｜User｜>Hi<｜Assistant｜>Hello<｜end▁of▁sentence｜>"` | `"用户：Hi助手：Hello"` |
| L42 | `"<｜Assistant｜>Hello<｜end▁of▁sentence｜><｜Tool｜>Search results<｜end▁of▁toolresults｜>"` | `"助手：Hello工具：Search results"` |
| L45 | `"<｜Tool｜>Search results<｜end▁of▁toolresults｜><｜User｜>How are you"` | `"工具：Search results用户：How are you"` |

注意：tool 角色现在会替换为拦截通知。但 test 中仍然期望 content 验证，需要适配新格式。

## 4. internal/util/util_edge_test.go

| 行号 | 旧值 | 新值 |
|------|------|------|
| L165 | `"<｜begin▁of▁sentence｜>"` | `"用户："` |
| L172 | `"<｜User｜>"` | `"用户："` |
| L178 | 移除端标记检查 | 改为检查计数 0 |
| L189 | `"<｜Assistant｜>"` | `"助手："` |
| L192 | `"<｜end▁of▁sentence｜>"` | 移除 |
| L195 | `"<｜end▁of▁sentence｜>"` | 改为检查助手：计数 |
| L198 | `"<｜Assistant｜>Hello!<｜end▁of▁sentence｜>"` | `"助手：Hello!"` |

## 5. internal/httpapi/openai/deps_injection_test.go

L106: `"<｜User｜>hi"` → `"用户：hi"`
