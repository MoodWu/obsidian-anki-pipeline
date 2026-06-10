# Obsidian Anki Pipeline

将 **Obsidian 中的英文生词自动转换为 Anki 记忆卡片** 的工具。

核心目标：

> 把「阅读中遇到的生词」自动沉淀为「可复习的记忆」

---

## 项目结构

```
your_dir/
├── dict/                  # 每个单词的独立释义文件（.md）
│   ├── trigger.md
│   ├── hook.md
│   └── ...
├── dictionary.json        # 所有词条的结构化数据
├── lemma.json             # 词形 → 词根映射缓存
└── log.json               # 处理批次日志（scan/add/batch 自动生成）
```

---

## 命令一览

| 命令 | 用途 | 需要 AI | 需要 Anki |
|---|---|---|---|
| `scan` | 扫描 Obsidian 目录，提取高亮生词 | ✅ | ✅ |
| `add` | 手动添加单词 | ✅ | ✅ |
| `batch` | 从文件批量添加单词 | ✅ | ✅ |
| `export` | 导出词典为 Markdown | ❌ | ❌ |
| `sync-anki` | 全量同步词典到 Anki（自动跳过已存在） | ❌ | ✅ |
| `log` | 查看历史处理批次 | ❌ | ❌ |
| `resync` | 按批次重新推送 Anki | ❌ | ✅ |

---

## 命令详解

### scan — 扫描目录

扫描 Obsidian 笔记目录，提取所有 `==高亮==` 标记的生词。

```bash
dict scan <dir> [选项]
```

**功能流程：**

1. 同步 `dict/` 文件夹与 `dictionary.json`（双向同步）
2. 扫描所有 `.md` 文件，查找 `==word==` 标记
3. 对每个生词：调用 AI 生成词条 → 创建 `.md) 释义文件 → 推送 Anki → 替换原文为 Obsidian 链接
4. 原文中的 `==running==` 会被替换为 `[[dict/run|running]]`（链接指向词根，显示原文形式）
5. 自动记录处理日志到 `log.json`

**选项：**

| 参数 | 说明 | 默认值 |
|---|---|---|
| `<dir>` | 要扫描的目录路径 | 必填 |
| `--dry-run` | 只预览，不实际写文件 | 关闭 |
| `--provider=` | AI 提供商：`openai` 或 `ollama` | `ollama` |
| `--model=` | AI 模型名称 | `deepseek-chat` |
| `--api-key=` | 在线 AI 的 API Key（用 ollama 时不需要） | 无 |
| `--anki-deck=` | Anki 牌组名称（必须已存在） | `背单词` |
| `--anki-model=` | Anki 卡片模板名称（须为问答题类型） | `记录单词` |

**示例：**

```bash
# 使用在线 AI
dict scan X:\km\Kent\English --provider=openai --model=deepseek-chat --api-key=sk-xxx --anki-deck=English

# 使用本地 ollama
dict scan X:\km\Kent\English --provider=ollama --model=qwen2.5:3b

# 只预览不写入
dict scan X:\km\Kent\English --dry-run
```

---

### add — 手动添加单词

```bash
dict add <word1> [word2 ...] [选项]
```

逐个处理单词，生成词条并推送 Anki。支持同时添加多个词。

**示例：**

```bash
dict add trigger motivation
dict add "slot machine" --provider=openai --api-key=sk-xxx
```

---

### batch — 文件批量添加

```bash
dict batch <file> [选项]
```

从文本文件逐行读取单词，批量处理。每行一个单词，空行和 `#` 开头的注释行自动跳过。

**示例：**

```bash
dict batch words.txt --provider=openai --api-key=sk-xxx
```

**words.txt 格式：**

```text
# 今天的生词
trigger
motivation
slot machine
hook
```

---

### export — 导出词典

```bash
dict export <file>
```

将 `dictionary.json` 导出为可读的 Markdown 文件。

**示例：**

```bash
dict export vocabulary.md
```

---

### sync-anki — 全量同步到 Anki

```bash
dict sync-anki [--anki-deck=牌组名] [--anki-model=模板名]
```

将 `dictionary.json` 中所有词条推送到 Anki。运行前先通过 AnkiConnect 查询已有词条，**自动跳过已存在的**，只推送缺失的。

**示例：**

```bash
dict sync-anki
dict sync-anki --anki-deck=English
```

**输出示例：**

```text
Anki 中已有 5 个词条
[SYNC] motivation
[SYNC] obsession
sync-anki done: 2 new, 7 skipped, 0 failed, total 9, 1.50s
```

---

### log — 查看历史批次

```bash
dict log
```

列出 `log.json` 中记录的所有处理批次，显示批次 ID、时间、命令类型、单词数量和状态统计。

**输出示例：**

```text
[20260607-214724] scan 2026-06-07 21:47:24 | 38 words: 35 new, 3 skip, 0 fail
[20260606-110819] scan 2026-06-06 11:08:19 | 17 words: 17 new, 0 skip, 0 fail
```

---

### resync — 按批次重新推送 Anki

```bash
dict resync <batch-id> [--anki-deck=牌组名] [--anki-model=模板名]
```

根据 `log.json` 中某个批次的单词列表，重新推送到 Anki。自动跳过该批次中 `fail` 状态的词。

优先使用命令行指定的 deck/model，未指定时回退到该批次记录的配置，都没有则用默认值。

**示例：**

```bash
# 重推某个批次
dict resync 20260607-214724

# 指定牌组
dict resync 20260607-214724 --anki-deck=English

# 指定牌组和模板
dict resync 20260607-214724 --anki-deck=English --anki-model=单词卡
```

---

## 通用选项

所有需要 AI 的命令（scan / add / batch）共享以下参数：

| 参数 | 说明 | 环境变量 | 默认值 |
|---|---|---|---|
| `--provider=` | `openai` 或 `ollama` | `AI_PROVIDER` | `ollama` |
| `--model=` | AI 模型名 | `AI_MODEL` | `deepseek-chat` |
| `--api-key=` | API Key | `OPENAI_API_KEY` | 无 |
| `--anki-deck=` | Anki 牌组名 | `ANKI_DECK` | `背单词` |
| `--anki-model=` | Anki 模板名 | `ANKI_MODEL` | `记录单词` |

**优先级：** 命令行参数 > 环境变量 > 默认值

**provider 说明：**
- `openai`：使用 OpenAI 兼容接口（如 DeepSeek），需要 `--api-key`
- `ollama`：使用本地 Ollama，需先启动 Ollama 服务

---

## log.json 格式

`log.json` 是一个 JSON 数组，每个元素代表一次处理批次。

### 批次结构（BatchLog）

```json
{
  "batch_id": "20260607-214724",
  "created_at": "2026-06-07 21:47:24",
  "command": "scan",
  "anki_deck": "English",
  "anki_model": "记录单词",
  "words": [...]
}
```

| 字段 | 类型 | 说明 |
|---|---|---|
| `batch_id` | string | 批次唯一 ID，格式 `YYYYMMDD-HHMMSS`，取该批次首个词的处理时间 |
| `created_at` | string | 批次开始时间，可读格式 |
| `command` | string | 触发命令：`scan`、`add`、`batch` |
| `anki_deck` | string | 该批次使用的 Anki 牌组名，空则用默认值 |
| `anki_model` | string | 该批次使用的 Anki 模板名，空则用默认值 |
| `words` | array | 该批次处理的单词列表 |

### 单词结构（WordLog）

```json
{
  "raw": "running",
  "word": "run",
  "status": "new"
}
```

| 字段 | 类型 | 说明 |
|---|---|---|
| `raw` | string | 原文中的原始形式（如 `running`、`went`） |
| `word` | string | 词典词头 / 词根形式（如 `run`、`go`） |
| `status` | string | 处理结果 |

**status 取值：**

| 值 | 含义 |
|---|---|
| `new` | 新生成词条，已推送 Anki |
| `skip` | 词典中已存在，跳过处理（仍尝试推送 Anki） |
| `fail` | 处理失败（AI 调用出错等） |

---

## 单词释义文件格式（dict/*.md）

每个单词生成一个独立的 Markdown 文件，存放在 `dict/` 目录下：

```markdown
---
aliases: []
---

# trigger

/trɪˈɡər/

## Meaning
verb
To cause something to happen or exist.
引发，触发

## Example
The news triggered a strong reaction.

## Translation
这条新闻引发了强烈的反应。

## Cloze
Certain smells can be ____ for people with PTSD.

## Note
▸ 构词法：trigger (v.) + -ing (现在分词后缀)
```

| 区域 | 说明 |
|---|---|
| `aliases` | Obsidian 别名，用于搜索 |
| `# word` | 标题，为词根形式 |
| `/phonetic/` | 国际音标 |
| `## Meaning` | 词性 + 英文释义 + 中文翻译 |
| `## Example` | 英文例句 |
| `## Translation` | 例句中文翻译 |
| `## Cloze` | 填空句（用 `____` 标记填空位置，用于 Anki） |
| `## Note` | 补充笔记（构词法、同根词、用法说明等） |

---

## 使用流程

### 1. 在 Obsidian 中阅读并标记生词

```text
The news ==triggered== a strong reaction.
```

### 2. 运行 scan

```bash
dict scan X:\km\Kent\English --provider=openai --model=deepseek-chat --api-key=sk-xxx --anki-deck=English
```

工具会：
- 提取高亮词 → AI 生成词条 → 创建 `dict/trigger.md`
- 原文变为 `[[dict/trigger|triggered]]`（Obsidian 内链，悬停可查看释义）
- 自动推送到 Anki
- 记录批次日志

### 3. 在 Anki 中复习

打开 Anki，新卡片已自动导入，正面为单词+音标，背面为释义/例句/笔记/填空。

### 4. 在 Obsidian 中回顾

原文中的高亮词已变为可点击的链接，按住 Ctrl 鼠标悬停可弹出释义预览。

### 5. 补推 Anki（如需要）

```bash
# 查看历史批次
dict log

# 按批次重推
dict resync 20260607-214724

# 全量补推（自动去重）
dict sync-anki
```

---

## 前置要求

- **Anki**：需安装并运行 [AnkiConnect](https://ankiweb.net/shared/info/2055492159) 插件（`localhost:8765`）
- **AI 服务**（二选一）：
  - 在线：OpenAI 兼容接口（如 DeepSeek），需要 API Key
  - 本地：[Ollama](https://ollama.com/)，需先启动服务并拉取模型
- **Obsidian**：用于阅读和标记生词

---

## 注意事项

- AI 生成内容可能不准确，建议抽样检查
- 生词标记必须使用 `==word==` 双等号语法
- Anki 牌组和模板必须提前在 Anki 中创建好
- `scan` 会修改原始笔记文件（将高亮替换为链接），建议先备份或使用版本控制
