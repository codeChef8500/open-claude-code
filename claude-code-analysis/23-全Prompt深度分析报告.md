# claude-code-main 全 Prompt 深度分析报告

> 分析范围：`src/constants/prompts.ts`、`src/coordinator/coordinatorMode.ts`、`src/tools/AgentTool/`、所有工具 `prompt.ts`、`src/memdir/`、`src/utils/systemPrompt.ts`

---

## 一、Prompt 架构总览

### 构建管线

```
getSystemPrompt() — constants/prompts.ts
│
├─ [静态段 · 全局缓存 scope:'global']
│   1. getSimpleIntroSection()         身份定义 + CYBER_RISK
│   2. getSimpleSystemSection()        6条系统规则
│   3. getSimpleDoingTasksSection()    任务执行规范（最复杂）
│   4. getActionsSection()             行动谨慎原则
│   5. getUsingYourToolsSection()      工具优先级规范
│   6. getSimpleToneAndStyleSection()  语调风格
│   7. getOutputEfficiencySection()    输出效率
│   __SYSTEM_PROMPT_DYNAMIC_BOUNDARY__ ← 缓存分界线
│
└─ [动态段 · systemPromptSection() 按名缓存]
    8.  session_guidance       会话专属指导
    9.  memory                 持久记忆
    10. ant_model_override     内部模型覆盖
    11. env_info_simple        运行环境信息
    12. language               语言偏好
    13. output_style           输出风格
    14. mcp_instructions [DANGEROUS_uncached]  MCP指令
    15. scratchpad             临时目录
    16. frc                    函数结果清理
    17. summarize_tool_results 工具结果摘要
    18. token_budget [feature] Token预算
    19. brief [KAIROS]         简报指令
```

### 缓存架构原理

`SYSTEM_PROMPT_DYNAMIC_BOUNDARY` 将 prompt 分成两部分：
- **边界前**：跨用户/组织可共享的全局缓存（`scope: 'global'`），内容完全静态
- **边界后**：每段独立缓存（`systemPromptSection(name, compute)`），`/clear` 或 `/compact` 时失效
- **`DANGEROUS_uncachedSystemPromptSection`**：每轮重新计算，会破坏缓存（须说明原因）

---

## 二、静态段逐段解析

### 1. getSimpleIntroSection — 身份定义

```
You are an interactive agent that helps users with software engineering tasks.
IMPORTANT: Assist with authorized security testing... [CYBER_RISK_INSTRUCTION]
IMPORTANT: You must NEVER generate or guess URLs...
```

**设计要点：**
- 角色描述极简，为 OutputStyle 覆盖预留空间
- `CYBER_RISK_INSTRUCTION` 受 Safeguards 团队保护，**禁止随意修改**（文件顶部有明确警告）
- URL 限制防止幻觉链接

`CYBER_RISK_INSTRUCTION` 内容：
> 协助授权安全测试、防御性安全、CTF 和教育场景。拒绝 DoS、大规模攻击、供应链攻击、检测规避等请求。双用途工具（C2框架、凭证测试、漏洞开发）需要明确授权上下文。

---

### 2. getSimpleSystemSection — 系统规则（6条）

| # | 规则 | 设计目的 |
|---|------|---------|
| 1 | Markdown 格式输出 | 终端等宽字体渲染 |
| 2 | 工具权限提示透传，拒绝后不重试相同调用 | 用户控制权 |
| 3 | `<system-reminder>` 标签说明 | 隔离系统注入 vs 用户消息 |
| 4 | 外部数据 prompt injection 警告 | 安全防御 |
| 5 | Hooks 反馈当用户消息处理 | 支持用户自定义 shell 钩子 |
| 6 | 自动上下文压缩说明 | 解除"对话太长"焦虑 |

---

### 3. getSimpleDoingTasksSection — 任务执行规范（最复杂段）

**过度工程防御三原则：**
- 不增加未被要求的功能、重构、"改进"
- 不为不可能发生的场景添加错误处理/回退/验证
- 不为一次性操作创建辅助函数/工具/抽象

**`USER_TYPE === 'ant'` 内部专属规则（构建时常量折叠）：**
- 注释规范：只在 WHY 非显而易见时才写（当前模型版本 Capybara v8 过度注释问题临时补丁，有 `@[MODEL LAUNCH]` 标记）
- 诚实报告：不能把失败说成通过，不能把未完成说成完成
- 协作者姿态：发现相邻 bug 或用户基于误解时主动告知
- 完成验证：报告完成前必须运行验证，无法验证时显式说明

**`@[MODEL LAUNCH]` 标注**：代码中有 5 处此标记，表示模型发布时需要更新的位置（如知识截止日期、最新模型 ID）。

---

### 4. getActionsSection — 行动谨慎原则

核心概念：**reversibility（可逆性）** + **blast radius（影响范围）**

需要确认的4类高风险操作：
1. **破坏性**：`rm -rf`、drop 表、force reset、杀进程
2. **难撤销**：force push、amend 已发布 commit、降级依赖、修改 CI/CD
3. **影响他人**：push 代码、创建/关闭 PR、发 Slack 消息、修改共享基础设施
4. **第三方上传**：粘贴板/图表工具（可能被缓存/索引）

**关键设计细节：**
> "用户批准一次 git push 不代表授权所有后续 push" — 授权范围不可泛化，须每次确认

---

### 5. getUsingYourToolsSection — 工具优先级

工具优先级层次：
```
Read/Edit/Write/Glob/Grep（专用工具）> Bash（兜底）
```

`embedded` 分支：ant 内部构建用 `bfs`/`ugrep` 替换 Glob/Grep，prompt 内容相应切换。

**并行调用指导：**
> "独立的工具调用用一条消息并行发出；有依赖的串行执行"

---

### 6. getOutputEfficiencySection — 两套输出规范

| 维度 | ant 内部版 | 外部版 |
|------|-----------|--------|
| 风格 | 新闻倒金字塔，流畅散文，避免符号/破折号 | 直接简洁 |
| 更新时机 | 发现重要信息、改变方向、关键进展时 | 需要决策/里程碑/阻塞时 |
| 受众假设 | 假设用户已离开、失去上下文，冷启动式说明 | 简短直接 |
| 专业词汇 | 展开缩写，解释术语，关注读者理解 | 不额外说明 |

---

## 三、动态段详解

### 7. Memory Prompt — memdir.ts

**记忆系统文件结构：**
```
~/.claude/projects/<slug>/memory/
├── MEMORY.md          ← 索引文件（最多200行/25KB，超限截断警告）
├── user_role.md       ← 用户偏好
├── feedback_xxx.md    ← 行为纠偏
├── project_xxx.md     ← 项目上下文
└── logs/YYYY/MM/      ← KAIROS 日志模式（append-only）
```

**四种记忆类型（封闭分类法）：**

| 类型 | 用途 | 保存时机 | 特殊规则 |
|------|------|---------|---------|
| `user` | 用户角色/偏好/技能水平 | 学到用户背景信息时 | 不写负面评价 |
| `feedback` | 行为纠偏和确认 | 纠正或确认时 | **同时保存纠错和确认**（只保存纠错会过于谨慎） |
| `project` | 项目决策/背景/动机 | 知道谁在做什么/为什么/何时 | 相对日期转绝对日期 |
| `reference` | 外部系统指针 | 学到外部资源位置时 | 指向 Linear/Slack/Grafana 等 |

**feedback 类型的结构规范：**
```
规则本身
**Why:** 用户给出的原因（过去的事故或强烈偏好）
**How to apply:** 何时/何处此规范生效
```
> 设计目的：只知道规则不够，知道**为什么**才能处理边界情况

**不应保存的内容（即使用户要求也不保存）：**
- 可从代码推导的内容（代码模式、架构、文件路径）
- git 历史（`git log`/`git blame` 更权威）
- 调试方案（fix 在代码里，背景在 commit message 里）
- CLAUDE.md 中已有内容
- 当前对话的临时状态

**KAIROS 模式（长期 session）：**
改为 append-only 日志模式，每天一个文件（`logs/YYYY/MM/YYYY-MM-DD.md`），夜间由 `/dream` skill 提炼成 MEMORY.md + topic 文件。

**记忆验证（TRUSTING_RECALL_SECTION）：**
- 记忆中提到的文件/函数/Flag 是"当时存在"的声明，现在可能已删除
- 推荐前必须 `grep`/`ls` 验证
- 标题用 "Before recommending from memory"（A/B 测试：3/3 效果）比 "Trusting what you recall"（0/3）更好

---

### 8. computeSimpleEnvInfo — 环境信息段

注入内容：
- 主工作目录 + 是否 git repo
- 额外工作目录列表
- Platform / Shell / OS Version
- 模型 ID + 知识截止日期
- 最新 Claude 模型族信息（帮助模型推荐最新版本）
- Claude Code 可用平台列表

**特殊逻辑：**
- `UNDERCOVER` 模式：完全隐藏模型名/ID（防内部代号泄漏到公开 PR/commit）
- Windows 平台：Shell 行追加 "use Unix syntax, not Windows"
- Worktree：注明是 isolated copy，禁止 cd 到原 repo root

---

### 9. getProactiveSection — KAIROS 自主模式

用于"永动 Agent"模式，核心机制：

**`<tick>` 心跳机制：**
- 定期唤醒，每个 tick 携带当前时间
- 多个 tick 可合批成一条消息，只处理最新的

**`SLEEP_TOOL_NAME` 控制等待间隔：**
- 平衡 API 成本 vs 提示缓存5分钟过期
- 无事可做时**必须** sleep，禁止输出 "still waiting"

**terminalFocus 感知：**
- 用户不在（Unfocused）→ 自主行动，做决策、提交、push
- 用户在看（Focused）→ 更协作，暴露选择，确认大改动

**行动偏向原则：**
> "A good colleague faced with ambiguity doesn't just stop — they investigate, reduce risk, and build understanding."

---

## 四、Agent 子系统 Prompt

### Agent Tool Prompt — AgentTool/prompt.ts

三种模式的 prompt 差异：

| 模式 | 特征 |
|------|------|
| Coordinator 模式 | 只显示核心共享段（详细指导已在 coordinator system prompt 中） |
| Fork 模式 | 增加 "When to fork" 段，fork 继承父上下文，prompt 是指令而非背景 |
| 标准模式 | 完整段落：agent 列表 + when NOT to use + 用法注意 + 示例 |

**Fork 模式核心约束：**
- **Don't peek**：不要 Read fork 的输出文件，等待完成通知
- **Don't race**：不要猜测/预测 fork 结果，通知到了再说
- **Writing a fork prompt**：fork 继承上下文，prompt 是指令而非背景说明

**Agent 列表注入方式：**
- 默认：内联在工具描述中（但变更会破坏 tools-block 缓存）
- `tengu_agent_list_attach` 功能门：改为 `agent_listing_delta` attachment 注入，保持工具描述静态

---

### Coordinator System Prompt — coordinatorMode.ts

完整多 Agent 编排系统，6个核心章节：

**1. 角色定义：**
- 编排者，不是执行者
- 直接回答能处理的问题，不委派给 worker
- Worker 结果是内部信号，不是对话伙伴（不要感谢或确认）

**2. 工具：**`Agent`、`SendMessage`、`TaskStop`、`subscribe_pr_activity`

**3. Workers：**
- 标准 worker 有全套工具 + MCP + Skill
- Simple 模式只有 Bash/Read/Edit

**4. 任务工作流（并发策略）：**

| 阶段 | 执行者 | 并发规则 |
|------|--------|---------|
| Research | Workers（并行） | 自由并行 |
| Synthesis | **Coordinator**（必须自己理解） | 串行 |
| Implementation | Workers | 同一文件集串行 |
| Verification | Workers | 可与不同文件区域的 Implementation 并行 |

**5. 编写 Worker Prompt 规范（最重要章节）：**

**Synthesis 原则**（禁止 lazy delegation）：
```
❌ 错误："Based on your findings, fix the auth bug"
❌ 错误："The worker found an issue, please fix it"
✅ 正确："Fix the null pointer in src/auth/validate.ts:42. The user field on Session is
          undefined when sessions expire but the token remains cached. Add a null check
          before user.id access — if null, return 401. Commit and report the hash."
```

**Continue vs Spawn 决策矩阵：**

| 情况 | 操作 | 原因 |
|------|------|------|
| 研究探索了需要编辑的文件 | Continue（SendMessage） | Worker 已有这些文件的上下文 |
| 研究广泛但实现聚焦 | Spawn fresh | 避免拖拽探索噪音 |
| 纠正失败或扩展近期工作 | Continue | Worker 有错误上下文 |
| 验证不同 worker 刚写的代码 | Spawn fresh | 验证者应用新鲜眼光 |
| 整个方向都错了 | Spawn fresh | 错误上下文会污染重试 |
| 完全无关的任务 | Spawn fresh | 无可复用上下文 |

**6. 完整示例会话**（带 `<task-notification>` XML 格式）

---

### 五个内置 Agent 对比

#### general-purpose
```
You are an agent for Claude Code. Complete the task fully—don't gold-plate,
but don't leave it half-done. Respond with a concise report — the caller will
relay this to the user, so it only needs the essentials.
```
- 工具：全部
- 模型：默认子 agent 模型

#### Explore（代码库搜索专家）
```
You are a file search specialist. This is a READ-ONLY exploration task.
STRICTLY PROHIBITED: creating, modifying, deleting, moving files, even /tmp.
```
- 工具：仅只读（Glob/Grep/Read/Bash只读命令）
- 模型：外部用户用 Haiku（速度优先），ant 用 inherit
- 额外优化：`omitClaudeMd: true`（不需要项目约定）

#### Plan（软件架构师）
```
You are a software architect. READ-ONLY planning task.
PROHIBITED: any file creation/modification/deletion.
Your role is EXCLUSIVELY to explore and design plans.
```
- 必须输出 "Critical Files for Implementation" 列表
- 模型：inherit

#### verification（破坏性测试专家）

这是**设计最精密**的 Agent，核心设计是**反自我欺骗**：

```
You have two documented failure patterns:
1. Verification avoidance: finding reasons not to run checks
2. Being seduced by the first 80%: seeing a passing test suite and feeling
   inclined to pass it

RECOGNIZE YOUR OWN RATIONALIZATIONS:
- "The code looks correct based on my reading" — reading is not verification. Run it.
- "The implementer's tests already pass" — the implementer is an LLM. Verify independently.
- "This is probably fine" — probably is not verified. Run it.
- "I don't have a browser" — did you check for mcp__claude-in-chrome__*?
- "This would take too long" — not your call.
```

**输出格式规范（每条 Check 必须包含）：**
```
### Check: [what you're verifying]
**Command run:** [exact command]
**Output observed:** [copy-paste, not paraphrased]
**Result: PASS** (or FAIL with Expected vs Actual)
```

**三态 Verdict：**
- `VERDICT: PASS` / `VERDICT: FAIL` / `VERDICT: PARTIAL`（仅环境限制时）
- 格式严格：不加 markdown bold、标点、变体

按变更类型的验证策略（前端/后端API/CLI/基础设施/库/Bug修复/移动端/数据管道/数据库迁移/重构）各有专项检查清单。

#### claude-code-guide（Claude 产品专家）
- 工具：WebFetch + WebSearch + 本地文件搜索
- 模型：Haiku
- `getSystemPrompt()` 动态注入用户当前配置：自定义技能、自定义 Agent、MCP 服务器、插件命令、settings.json
- 查询官方文档优先（`code.claude.com/docs` 和 `platform.claude.com/llms.txt`）

---

## 五、工具级 Prompt 详解

### Bash Tool

**外部版 Git SOP（300+ 行完整流程）：**

`git commit` 四步流程：
1. 并行：`git status` + `git diff` + `git log`（了解提交风格）
2. 分析所有变更，起草 commit message（聚焦 why 而非 what）
3. 并行：git add + git commit（HEREDOC 格式）+ git status 验证
4. 若 pre-commit hook 失败：修复问题，创建**新** commit（不 amend）

关键约束：
- NEVER 跳过 hooks（`--no-verify`）
- NEVER force push main/master
- NEVER 未被明确要求时 commit
- 始终用 HEREDOC 传 commit message（格式保证）

**Sandbox 感知提示：**
- 动态注入文件系统读写 allowlist/denylist 的 JSON 描述
- `$TMPDIR` 规范化（去除 UID 路径，保持跨用户 prompt cache 稳定）
- 两种 sandbox 模式：可覆盖 vs 策略强制不可覆盖

**sleep 滥用防御：**
> 不要在等待循环里 sleep，使用 `run_in_background` + 等通知

### File Read Tool

```
Reads a file from the local filesystem.
- file_path 必须是绝对路径
- 默认读最多 2000 行
- 支持 offset + limit（已知位置时只读该部分）
- 支持图片（多模态）、PDF（最多20页/次）、Jupyter notebooks
- FILE_UNCHANGED_STUB：文件未变化时返回提示，避免重复传输
```

### File Edit Tool

```
Performs exact string replacements in files.
- 必须先 Read 再 Edit（未读先编辑会报错）
- old_string 必须在文件中唯一（否则提供更多上下文或用 replace_all）
- 保留精确缩进（tab/space）
- ant 版额外规则：old_string 通常只需 2-4 行，避免包含 10+ 行上下文
```

### File Write Tool

```
- 覆盖写时必须先 Read
- 优先用 Edit（只传 diff）；整体重写才用 Write
- NEVER 创建 *.md 或 README 文件（除非明确要求）
- 不写 emoji（除非明确要求）
```

### Glob Tool

```
Fast file pattern matching, supports **/*.js, src/**/*.ts
Returns matching paths sorted by modification time
```

### Grep Tool

```
Built on ripgrep.
- ALWAYS 用 Grep，NEVER 直接调 grep/rg 命令
- 支持完整 regex 语法
- 输出模式：content / files_with_matches / count
- Go 代码中花括号需转义：interface\{\} 而非 interface{}
```

### WebFetch Tool — 二级模型 Prompt

两种内容处理策略：
- **预审核域名**：自由引用，提供详细文档
- **非预审核域名**：引用 ≤125字符，使用引号，不重现歌词，不评论合法性

### WebSearch Tool

```
- 结果为止：每次回答必须包含 Sources: 章节（MANDATORY）
- 当前月份注入：防止搜索使用错误年份
- 仅在美国可用
```

### AskUserQuestion Tool

```
- 支持多选（multiSelect: true）
- 推荐选项放第一位并标注 "(Recommended)"
- Plan mode：不要问"计划好了吗？"，用 ExitPlanMode 工具
- 提问时不要引用"计划"（用户还看不到计划内容）
- preview 字段：ASCII mockup / code 片段对比，仅单选支持
```

### TodoWrite Tool

**何时使用（任一条件满足）：**
1. 需要 3+ 个明确步骤的复杂任务
2. 用户提供了多个任务列表
3. 用户明确要求
4. 收到新指令后立即记录

**何时不用：**
- 单个简单任务
- 纯对话/信息类请求
- 3步以内可完成的小事

**状态管理规则：**
- 同时只有 1 个 `in_progress`
- 完成立即标记，不批量标记
- 每条任务需提供两种形式：`content`（祈使句）和 `activeForm`（进行时）

### Skill Tool

**Token 预算管理：**
```
预算 = 上下文窗口 × 4 chars/token × 1% ≈ 8000 chars
每条描述上限 250 字符
bundled 技能：始终保留完整描述
用户技能：按比例截断
极端情况：只显示名称
```

**关键行为约束：**
- 匹配到技能就必须调用，这是 **BLOCKING REQUIREMENT**
- 看到 `<command-name>` 标签则技能已加载，直接遵循指令，不再调用

---

## 六、Brief Tool（KAIROS 通信层）

设计背景：KAIROS 自主模式下，普通文本在"detail view"中，用户不一定打开。`SendUserMessage` 是用户**真正会看到**的频道。

**三种 status：**
- `normal`：回应用户刚问的
- `proactive`：主动发起（后台任务完成、发现阻塞、需要用户输入）

**通信模式：**
```
需要查询时：ack("On it — checking...") → work → result
长期工作：ack → work → checkpoint（有信息量时）→ result
```

> checkpoint 要有信息量：做了决策、遇到意外、跨越阶段边界。跳过填充词 "running tests..."。

---

## 七、System Prompt 优先级仲裁

`buildEffectiveSystemPrompt()` 五级优先级（从高到低）：

```
0. overrideSystemPrompt     完全替换（loop 模式）
         ↓
1. Coordinator 模式 prompt  编排专用
         ↓
2. Agent 定义 prompt        主线程 Agent
   - PROACTIVE 模式：追加到默认 prompt 后
   - 普通模式：替换默认 prompt
         ↓
3. customSystemPrompt       --system-prompt 参数
         ↓
4. defaultSystemPrompt      标准 Claude Code prompt
         ↓
+ appendSystemPrompt        始终追加（任何模式下）
```

---

## 八、核心设计模式总结

### 1. Build-time DCE（死代码消除）

```typescript
const proactiveModule = feature('PROACTIVE') ? require(...) : null
```

所有 `feature()` 调用在 `bun:bundle` 时被常量折叠，未启用 feature 的代码完全从 bundle 移除。ant 内部构建和外部构建产生**两套不同逻辑**，实质上是两个不同的产品。

### 2. 条件编译的用户分层

```typescript
process.env.USER_TYPE === 'ant'  // 构建时常量
```

三种受众各自不同：
- **ant 内部**：更严格的代码规范、诚实报告、字数上限（≤25词/≤100词）
- **外部用户**：简洁友好
- **REPL 模式**：只保留 Task 工具指导

### 3. Prompt Injection 防御

- `<system-reminder>` 标签隔离：明确告知模型这些标签是系统自动插入的
- 外部数据警告：工具结果可能包含 prompt injection，发现须告知用户
- URL 生成限制：不猜 URL（防幻觉链接泄漏内部信息）

### 4. Agent 权限隔离

每类 Agent 有明确的权限边界，用 `disallowedTools` 强制实施：
- Explore/Plan：禁用所有写工具（包括 /tmp）
- Verification：禁用写工具（tmp 例外），禁用 Agent 工具（防链式托管）
- claude-code-guide：只有 WebFetch/WebSearch + 本地只读搜索

### 5. A/B 测试驱动的 Prompt 优化

源码注释中多处记录了 eval 结果：
- `TRUSTING_RECALL_SECTION` 标题：测试出 "Before recommending from memory" (3/3) 比 "Trusting what you recall" (0/3) 效果更好
- `numeric_length_anchors`：研究显示数值锚定（≤25词/≤100词）比定性说明"简洁"减少约 1.2% 输出 token
- `MEMORY_DRIFT_CAVEAT` 位置：放在独立章节比作为子条目效果更好（0/2 → 3/3）

### 6. 缓存优化是头等公民

多处注释说明了缓存考量：
- Agent 列表从工具描述移到 attachment：MCP/插件/权限变化会更新 agent 列表 → 工具描述变化 → 整个 tools-block 缓存失效
- `mcp_instructions` 设为 DANGEROUS_uncached：MCP 服务器随时连接/断开，每轮必须重新计算
- 工具描述中的 `claudeTempDir`：替换为 `$TMPDIR`（去除 UID 路径），跨用户共享缓存前缀
- `SYSTEM_PROMPT_DYNAMIC_BOUNDARY` 之前的静态内容：跨组织/用户的全局缓存

---

## 九、对 agent-engine 的参考价值

### 直接可借鉴的 Prompt 文本

| 来源 | 用途 |
|------|------|
| Verification Agent system prompt | `internal/agent/` 中的验证角色 |
| Coordinator `getCoordinatorSystemPrompt()` | `internal/agent/coordinator.go` 的 system prompt |
| `DEFAULT_AGENT_PROMPT` | 通用子 agent 的默认 prompt |
| Memory 的 `feedback` 类型结构 | agent-engine 的 CLAUDE.md/记忆模板 |
| `CYBER_RISK_INSTRUCTION` | 安全红线指令（可直接使用） |

### 架构层面参考

| 模式 | agent-engine 对应点 |
|------|-------------------|
| 静态/动态双轨缓存 | `internal/prompt/adapter.go` 的 system prompt 构建 |
| `disallowedTools` 而非只有 allowlist | `engine.Tool.IsEnabled()` 的权限模型 |
| `systemPromptSection` 按名缓存 | prompt 段的增量更新 |
| `DANGEROUS_uncached` 标注 | 哪些 prompt 段不可缓存需明确文档化 |
| Agent 身份隔离（Verification 不能写文件） | `internal/engine/tool_iface.go` 的 UseContext 权限 |

### 关键设计原则（可直接应用）

1. **Synthesis 原则**：Coordinator 必须自己理解研究结果，不能写 "based on your findings, fix it"
2. **Verification 的反自欺骗列表**：可直接移植到 agent-engine 的 verification agent prompt
3. **Memory 的禁存规则**：可从代码推导的内容不保存为记忆（防止记忆污染）
4. **授权范围不泛化**：单次批准不代表永久授权，须每次确认高风险操作
5. **工具偏好层次**：专用工具 > Bash 兜底，明确每种工具的使用边界
