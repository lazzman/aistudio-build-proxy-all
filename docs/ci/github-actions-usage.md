# GitHub Actions 工作流使用说明

本文档基于仓库内以下两个工作流文件编写，文中涉及的 **Variables / Secrets / workflow_dispatch inputs 键名** 仅引用工作流中实际出现的名称：

- Sync Upstream：（[`sync-upstream.yml`](../../.github/workflows/sync-upstream.yml)
- Docker Image：[`docker-image.yml`](../../.github/workflows/docker-image.yml)

---

## 适用范围

### 1) Sync Upstream（[`sync-upstream.yml`](../../.github/workflows/sync-upstream.yml)）

- **适用仓库**：适用于需要从上游仓库同步目标分支的当前仓库（不要求仓库必须是 fork）。
- **用途**：将“上游仓库”的目标分支同步到“当前仓库”的目标分支。
- **同步策略**（由 `merge_strategy` / `SYNC_MERGE_STRATEGY` 决定）：
  - `merge_request`：同步后推送临时分支，并自动创建 PR 到目标分支。
  - `direct`：同步后直接将变更推送到目标分支（使用 `git push --force-with-lease`）。
- **通知**：可选企业微信通知（由 `WECHAT_WORK_WEBHOOK_URL` 是否为空决定是否发送）。

### 2) Docker Image（[`docker-image.yml`](../../.github/workflows/docker-image.yml)）

- **用途**：构建并推送 Docker 镜像到 GitHub Container Registry（GHCR）。
- **镜像地址**：`ghcr.io/${{ github.repository }}`（形如 `ghcr.io/<owner>/<repo>`）。
- **构建平台**：`linux/amd64,linux/arm64`。
- **前提**：工作流在构建步骤中固定使用 `file: Dockerfile`，仓库根目录需存在 `Dockerfile` 文件。

---

## 必需配置（Variables / Secrets）

### Sync Upstream（[`sync-upstream.yml`](../../.github/workflows/sync-upstream.yml)）

- **无必需 Variables / Secrets**：该工作流对 `UPSTREAM_REPO_URL` / `TARGET_BRANCH` / `MERGE_STRATEGY` / `SYNC_BRANCH_PREFIX` 都提供了默认值（见下文“变量取值优先级”）。
- Token 行为（无需手动配置也可运行）：工作流会在 `secrets.PAT_TOKEN` 非空时使用它，否则使用 `github.token`。

### Docker Image（[`docker-image.yml`](../../.github/workflows/docker-image.yml)）

- **无需额外创建 Variables / Secrets**：工作流使用 GitHub 自动提供的 `secrets.GITHUB_TOKEN` 登录 `ghcr.io` 并推送镜像。
- **权限要求**：工作流顶部已声明 `permissions.contents: read` 与 `permissions.packages: write`，用于读取仓库内容并推送镜像到 GHCR。

---

## 可选配置（Variables / Secrets）

> 说明：本节列出的键名均来自工作流内的实际引用。

### Sync Upstream 可选 Variables（仓库级 Variables）

| 键名 | 用途 | 典型取值示例 |
|---|---|---|
| `SYNC_UPSTREAM_REPO_URL` | 覆盖“上游仓库 HTTPS URL” | `https://github.com/<upstream-owner>/<upstream-repo>.git` |
| `SYNC_TARGET_BRANCH` | 覆盖“目标分支” | `main` |
| `SYNC_MERGE_STRATEGY` | 覆盖“同步策略” | `merge_request` 或 `direct` |
| `SYNC_BRANCH_PREFIX` | 覆盖“临时分支前缀”（仅 `merge_request` 策略会用到临时分支） | `sync-upstream-` |

### Sync Upstream 可选 Secrets（仓库级 Secrets）

| 键名 | 用途 | 生效条件 |
|---|---|---|
| `PAT_TOKEN` | 作为工作流的有效 token（用于 `actions/checkout` 的 `token`、以及 `actions/github-script` 的 `github-token`） | 当 `secrets.PAT_TOKEN != ''` 为真时优先使用，否则回退到 `github.token` |
| `WECHAT_WORK_WEBHOOK_URL` | 企业微信通知 webhook | 当 `WECHAT_WORK_WEBHOOK_URL != ''` 时，成功/失败路径会发送通知 |

### Docker Image 可选 Variables / Secrets

- 当前工作流未引用额外的仓库级 Variables / Secrets（除 `secrets.GITHUB_TOKEN` 外）。

---

## 变量取值优先级（vars > workflow_dispatch inputs > default）

本节仅适用于 Sync Upstream（[`sync-upstream.yml`](../../.github/workflows/sync-upstream.yml)），其在 job `env:` 中采用 `vars.xxx || inputs.xxx || default` 的取值模式：

| 运行时环境变量 | 优先级 1（仓库 Variables） | 优先级 2（手动触发 inputs） | 默认值 |
|---|---|---|---|
| `UPSTREAM_REPO_URL` | `SYNC_UPSTREAM_REPO_URL` | `upstream_repo_url` | `https://github.com/NikkeTryHard/aistudio-build-proxy-all.git` |
| `TARGET_BRANCH` | `SYNC_TARGET_BRANCH` | `target_branch` | `github.event.repository.default_branch` |
| `MERGE_STRATEGY` | `SYNC_MERGE_STRATEGY` | `merge_strategy` | `direct` |
| `SYNC_BRANCH_PREFIX` | `SYNC_BRANCH_PREFIX` | `sync_branch_prefix` | `sync-upstream-` |

补充：token 选择逻辑不属于上述三段优先级，但同样是“有则用之”的模式：

- 当 `PAT_TOKEN` 非空时使用 `PAT_TOKEN`；否则使用 `github.token`。

---

## 同步使用的合并方式（基于真实 git 操作）

> 本节仅基于 [`sync-upstream.yml`](../../.github/workflows/sync-upstream.yml) 的实际脚本行为，不做额外推断。

| 关注点 | 实际行为 | 关键行引用 |
|---|---|---|
| 默认同步策略 | 默认是 `direct`（可被仓库变量/手动输入覆盖） | [`workflow_dispatch.inputs.merge_strategy.default = direct`](../../.github/workflows/sync-upstream.yml:23)、[`MERGE_STRATEGY` 取值链 `vars -> inputs -> default`](../../.github/workflows/sync-upstream.yml:50) |
| 默认是否使用 `git rebase` | 当检测到上游有更新时，执行 `git rebase upstream/${TARGET_BRANCH}`；无更新则标记 `no_updates` 并结束 | [`HAS_UPDATES` 判定](../../.github/workflows/sync-upstream.yml:105)、[`git rebase`](../../.github/workflows/sync-upstream.yml:106)、[`REBASE_STATUS=no_updates`](../../.github/workflows/sync-upstream.yml:122) |
| `direct` 如何落地 | rebase 成功后，直接将 `HEAD` 强校验推送到目标分支：`git push --force-with-lease origin HEAD:${TARGET_BRANCH}` | [`direct` 分支判断](../../.github/workflows/sync-upstream.yml:107)、[`git push --force-with-lease`](../../.github/workflows/sync-upstream.yml:108) |
| `merge_request` 如何落地 | rebase 成功后创建临时分支并推送；随后在下一个 job 创建 PR（base=目标分支，head=临时分支） | [`TEMP_BRANCH` 生成](../../.github/workflows/sync-upstream.yml:111)、[`git checkout -b`](../../.github/workflows/sync-upstream.yml:112)、[`git push --set-upstream`](../../.github/workflows/sync-upstream.yml:113)、[`create_pull_request` 触发条件（仅 `merge_request`）](../../.github/workflows/sync-upstream.yml:161)、[`pulls.create`](../../.github/workflows/sync-upstream.yml:265) |
| rebase 冲突行为 | rebase 失败时执行 `git rebase --abort`，标记失败并 `exit 1`，工作流失败退出，不自动合并 | [`rebase` 失败分支](../../.github/workflows/sync-upstream.yml:116)、[`git rebase --abort`](../../.github/workflows/sync-upstream.yml:117)、[`SHOULD_FAIL=true`](../../.github/workflows/sync-upstream.yml:119)、[`exit 1`](../../.github/workflows/sync-upstream.yml:136) |
| 是否执行 merge commit | 当前脚本仅包含 rebase + push + PR 创建；未出现 `git merge` 或自动合并 PR 的逻辑 | [`Fetch, rebase and push by strategy` 脚本段](../../.github/workflows/sync-upstream.yml:68)、[`pulls.create`（仅创建 PR）](../../.github/workflows/sync-upstream.yml:265)、[`此 PR 由 GitHub Actions 自动创建，请审核后合并`](../../.github/workflows/sync-upstream.yml:262) |

---

## 触发方式与行为说明（schedule / workflow_dispatch / push / tag）

### 1) schedule（定时触发）

- **仅 Sync Upstream**（[`sync-upstream.yml`](../../.github/workflows/sync-upstream.yml)）配置了 `schedule`。
- Cron：`*/10 * * * *`（每10分钟执行一次）。
- 行为概述：
  1. 检出 `TARGET_BRANCH`。
  2. 添加/更新 `upstream` remote，拉取 `origin/${TARGET_BRANCH}` 与 `upstream/${TARGET_BRANCH}`。
  3. 计算差异：如果上游有新提交，则执行 `git rebase upstream/${TARGET_BRANCH}`。
  4. 根据策略：
     - `direct`：rebase 成功后直接推送到 `TARGET_BRANCH`。
     - `merge_request`：创建临时分支并推送，然后进入 PR 创建 job。

### 2) workflow_dispatch（手动触发）

#### Sync Upstream（[`sync-upstream.yml`](../../.github/workflows/sync-upstream.yml)）

该工作流定义了以下 `workflow_dispatch.inputs`（均为可选）：

| input 键名 | 含义 | 备注 |
|---|---|---|
| `upstream_repo_url` | 上游仓库 HTTPS URL | 会被 `vars.SYNC_UPSTREAM_REPO_URL` 覆盖（若后者存在） |
| `target_branch` | 目标分支 | 会被 `vars.SYNC_TARGET_BRANCH` 覆盖（若后者存在） |
| `merge_strategy` | 同步策略 | `merge_request` / `direct`；默认值为 `direct`；也会被 `vars.SYNC_MERGE_STRATEGY` 覆盖 |
| `sync_branch_prefix` | 临时分支前缀 | 会被 `vars.SYNC_BRANCH_PREFIX` 覆盖（若后者存在） |

#### Docker Image（[`docker-image.yml`](../../.github/workflows/docker-image.yml)）

- 该工作流支持手动触发，但未定义 `inputs`。
- 镜像 tag 由 `docker/metadata-action@v5` 生成，规则依赖 `github.ref`：
  - 当 `github.ref == 'refs/heads/main'` 时：生成 `main`、`latest`，并附加 `sha-<shortsha>`。
  - 当 `github.ref` 以 `refs/tags/v` 开头时：生成 `latest`、tag 同名（如 `v1.2.3`），并附加 `sha-<shortsha>`。

### 3) push（代码推送触发）

- **仅 Docker Image**（[`docker-image.yml`](../../.github/workflows/docker-image.yml)）配置了 `push` 触发：
  - 分支：仅 `main`
  - tag：`v*`

### 4) tag（打标签触发）

- Docker Image 的 tag 触发是 `push.tags: v*` 的一部分。
- 当 `github.ref` 为 tag 且匹配 `refs/tags/v*` 时，`docker/metadata-action@v5` 会生成：
  - `latest`
  - 当前 tag 同名镜像 tag（来自 `type=ref,event=tag`，例如 `v1.2.3`）
  - `sha-<shortsha>`（来自 `type=sha,prefix=sha-`）

---

## 首次配置最小步骤

1. 确认仓库中存在工作流文件：[`sync-upstream.yml`](../../.github/workflows/sync-upstream.yml)、[`docker-image.yml`](../../.github/workflows/docker-image.yml)。
2. （仅 Sync Upstream）确认仓库具备向目标分支或临时分支推送的权限（`PAT_TOKEN` 或 `github.token`）。
3. （可选）为 Sync Upstream 配置仓库 Variables（路径：Settings → Secrets and variables → Actions → Variables）：
   - `SYNC_UPSTREAM_REPO_URL`
   - `SYNC_TARGET_BRANCH`
   - `SYNC_MERGE_STRATEGY`
   - `SYNC_BRANCH_PREFIX`
4. （可选）为 Sync Upstream 配置仓库 Secrets（路径：Settings → Secrets and variables → Actions → Secrets）：
   - `PAT_TOKEN`
   - `WECHAT_WORK_WEBHOOK_URL`
5. 手动触发一次 Sync Upstream 以验证配置（Actions → Sync Upstream → Run workflow）：
   - `merge_strategy` 可保持默认 `direct`，或按需选择 `merge_request`。
   - 如同时配置了 `vars`，请以“变量取值优先级”章节为准判断最终生效值。
6. 观察 Sync Upstream 执行结果：
   - 若没有上游更新：日志会显示无更新（工作流不会创建分支/PR）。
   - 若选择 `merge_request` 且有更新：应出现临时分支（前缀来自 `SYNC_BRANCH_PREFIX` / `sync_branch_prefix` / 默认值）并创建 PR。
   - 若选择 `direct` 且有更新：应直接更新 `TARGET_BRANCH`。
7. 触发 Docker Image 构建：
   - 方式 A：push 到 `main`。
   - 方式 B：推送 tag（匹配 `v*`）。示例：
     ```bash
     git tag v0.1.0
     git push origin v0.1.0
     ```
   - 方式 C：手动触发（Actions → docker image → Run workflow）。
8. 验证 Docker 镜像推送结果：
   - 在 Actions 日志中确认 `Docker meta`、`Login to GHCR` 与 `Build and push` 成功。
   - 在 GHCR 中查看 `ghcr.io/<owner>/<repo>` 的 tags（按触发类型可能包含 `main` / `latest` / `v*` / `sha-*`）。

---

## 常见故障排查

> 建议优先从对应 workflow run 的日志入手：
> - Sync Upstream：关注 `Fetch, rebase and push by strategy` 步骤开头打印的“目标分支/上游仓库/同步策略”。
> - Docker Image：关注 `Docker meta` / `Login to GHCR` / `Build and push` 三个步骤。

1. **Sync Upstream 触发了但 `create_pull_request` job 显示 skipped**
   - 现象：workflow run 存在，`Sync from upstream` 执行完成，但 `Cleanup history and create pull request` 未执行。
   - 原因：`create_pull_request` 依赖条件未满足（如 `HAS_UPDATES != 'true'`、`REBASE_STATUS != 'success'` 或策略不是 `merge_request`）。
   - 处理：检查上一个 job 的输出与策略配置，重点确认 `HAS_UPDATES`、`REBASE_STATUS`、`EFFECTIVE_MERGE_STRATEGY`。

2. **Sync Upstream 报错：rebase 冲突导致失败**
   - 现象：`Fetch, rebase and push by strategy` 步骤失败，日志中出现 `git rebase` 相关错误。
   - 原因：`origin/${TARGET_BRANCH}` 与 `upstream/${TARGET_BRANCH}` 存在不可自动 rebase 的冲突。
   - 处理：需要人工在本地处理冲突（以 `TARGET_BRANCH` 为准）后再重试该 workflow。

3. **Sync Upstream 显示成功但没有创建 PR**
   - 现象：workflow run 成功，但仓库里没有新的 PR。
   - 常见原因与定位：
     - 没有上游更新：脚本会将 `HAS_UPDATES` 置为 `false`，此时不会创建临时分支/PR。
     - 策略不是 `merge_request`：只有当策略为 `merge_request` 且有更新时，`create_pull_request` job 才会运行。
   - 处理：确认 `MERGE_STRATEGY` 的最终生效值（见“变量取值优先级”），并确认上游确实有新提交。

4. **Sync Upstream push 失败（403/权限不足/受保护分支）**
   - 现象：在推送临时分支或直接推送目标分支时失败。
   - 原因：工作流推送行为依赖有效 token（`PAT_TOKEN` 或 `github.token`）与仓库分支策略。
   - 处理：
     - 如当前 token 权限不足：配置 `PAT_TOKEN` 作为更高权限的 token。
     - 如目标分支受保护且不允许强推：避免使用 `direct` 策略，改用 `merge_request`。

5. **企业微信通知未发送**
   - 现象：workflow 成功/失败但没有收到企业微信消息。
   - 原因：`WECHAT_WORK_WEBHOOK_URL` 为空时，通知步骤的 `if:` 条件不成立。
   - 处理：在仓库 Secrets 中配置 `WECHAT_WORK_WEBHOOK_URL`。

6. **Sync Upstream 清理逻辑误伤其他 PR/分支**
   - 现象：某些以相同前缀开头的历史同步 PR 被自动关闭，或同前缀分支被删除。
   - 原因：`create_pull_request` job 会清理所有 `head ref` 以 `SYNC_BRANCH_PREFIX` 开头的历史 PR/分支（排除本次 `TEMP_BRANCH`）。
   - 处理：通过 `SYNC_BRANCH_PREFIX`（vars 或 inputs）设置一个更具唯一性的前缀，避免与其他分支命名冲突。

7. **Docker Image 未触发**
   - 现象：push 后没有出现 workflow run。
   - 原因与处理：
     - push 的分支不是 `main`，且也未推送匹配 `v*` 的 tag：该工作流仅在 `main` 分支或 `v*` tag push 时自动触发。
     - push 的 tag 不匹配 `v*`：请使用如 `v1.2.3` 的 tag。

8. **Docker Image 登录 GHCR 失败**
   - 现象：`Login to GHCR` 步骤失败。
   - 原因：该步骤使用 `secrets.GITHUB_TOKEN` 作为 `password`；推送到 GHCR 需要对应的 `packages: write` 权限。
   - 处理：确认 workflow 顶部的 `permissions` 中包含 `packages: write`，并在失败日志中查看具体错误信息再针对性处理。

9. **Docker Image 构建失败：找不到 Dockerfile**
   - 现象：`Build and push` 步骤报错提示无法找到/读取 Dockerfile。
   - 原因：工作流显式指定 `file: Dockerfile`。
   - 处理：确保仓库根目录存在 `Dockerfile` 文件；否则需要调整工作流（此文档不涉及修改工作流）。

10. **Docker Image 产出的 tag 与预期不一致**
   - 现象：GHCR 中缺少 `main` 或缺少版本 tag，或仅看到 `sha-*`。
   - 原因：tag 生成与 `github.ref` 强绑定：
     - `refs/heads/main`：生成 `main` / `latest` / `sha-*`
     - `refs/tags/v*`：生成 `latest` / `<tag>` / `sha-*`
   - 处理：确认本次 run 的触发来源（分支 push / tag push / 手动触发所选分支）与上述规则一致。

11. **Docker Image 多架构构建失败（Buildx / QEMU）**
   - 现象：`Build and push` 在 `linux/amd64,linux/arm64` 构建阶段失败。
   - 原因：可能是 `Set up QEMU`、`Set up Docker Buildx` 或镜像构建本身失败。
   - 处理：按顺序检查 `Set up QEMU`、`Set up Docker Buildx`、`Build and push` 的日志定位具体失败点，再针对性处理。
