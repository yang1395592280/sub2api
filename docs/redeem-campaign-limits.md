# 福利活动兑换限领

## 使用方式

1. 打开管理端“兑换码”，点击“福利活动”，新建活动，填写名称和每账号限兑次数（默认 1，允许 1–1000 的整数）。
2. 生成余额码或抽奖券码时，点击“选择或新建活动”。选择已有活动即为该活动补发，所有批次共享限额。
3. 在“福利活动”中编辑活动，可修改名称和限兑次数；兑换码列表的“活动 #编号”可直接打开对应活动。
4. 新建活动重新计算领取额度。不关联活动的普通码沿用原有兑换规则。

同一账号在同一活动中成功兑换一张码计一次，余额码和抽奖券码合计计数。一张码发放多张抽奖券仍计一次。限额从 1 调为 3 时，已经领取 1 次的账号还能再领 2 次；调低不会撤回已有权益，已达到新限额的账号无法继续领取。

兑换失败不占额度、不消耗兑换码。超限时返回 HTTP 409，原因 `REDEEM_CAMPAIGN_LIMIT_REACHED`，其他账号仍可兑换被拒绝的码。删除已使用兑换码不会恢复活动额度。

当前按站内用户 ID 识别账号，不识别同一个人注册的多个账号。仅支持正数余额码和抽奖券码参与活动，邀请码、订阅码、并发码、充值履约等原有入口保持其既有用途。

## 管理 API

接口位于现有管理员鉴权、限流、审计和合规中间件内：

| 方法 | 路径（前缀 `/api/v1/admin/redeem-codes`） | 内容 |
|---|---|---|
| GET | `/campaigns?page=1&page_size=20&search=名称` | 活动分页列表，每页上限 100 |
| GET | `/campaigns/:id` | 活动详情 |
| POST | `/campaigns` | 新建活动 |
| PUT | `/campaigns/:id` | 修改名称、限额，不重置领取记录 |

创建、修改请求体示例：`{"name":"周五群福利","per_user_limit":3}`。写操作沿用项目的 `Idempotency-Key` 机制；前端对相同内容的失败重试复用操作键。

原生成接口 `POST /generate` 新增可选 `campaign_id`；不传时保持原行为。管理端兑换码响应增加可选 `campaign_id`，用户兑换/历史响应不暴露该内部字段。活动归属仅在生成时设置，不提供变更归属或删除活动的接口。

## 数据与并发

迁移 `247_redeem_campaign_limits.sql` 新增：

- `redeem_campaigns`：活动、每账号限额和创建/修改时间。
- `redeem_codes.campaign_id`：可空外键和索引，旧码保持 NULL，不回填历史活动。
- `redeem_campaign_claims`：以活动 ID + 用户 ID 为主键，独立保存已成功兑换的码数，不关联可被硬删除的兑换码。

兑换沿用现有数据库事务。在标记兑换码后，对活动配置取得 `FOR SHARE` 锁，再以唯一键和条件 UPSERT 原子增加计数。活动配置修改等待进行中的领取事务完成；计数、兑换码状态、余额/抽奖券权益一同提交或回滚。Redis 不可用不影响数据库限领机制；活动数据库操作失败则拒绝此次兑换。

## 上线与回退

- 新版本启动时由现有迁移机制应用新增结构。应先在测试环境确认迁移，再按既有发布方式上线；本次开发未操作生产数据库。
- 回退应用前，需要备份活动和领取记录，并暂停活动码兑换或在确认后禁用尚未使用的活动码。旧应用不识别活动限额，不能直接把仍可使用的活动码交给旧版本处理。
- 常规回退保留新增结构和领取计数，防止升级回来后丢失限领历史。不回填、不清空旧数据。
- 若明确决定永久撤销该能力，备份后可人工执行下面的结构清理。清理会删除领取历史，且已应用迁移有 checksum/版本记录，后续重新上线需新增迁移恢复，不能仅重启应用。以下 SQL 不由程序自动执行：

```sql
ALTER TABLE redeem_codes DROP COLUMN IF EXISTS campaign_id;
DROP TABLE IF EXISTS redeem_campaign_claims;
DROP TABLE IF EXISTS redeem_campaigns;
```

## 验证

```sh
cd backend
go test ./internal/service ./internal/repository ./internal/handler/admin ./internal/handler/dto ./internal/handler ./internal/server/routes ./migrations -count=1
go test -race -tags=integration ./internal/repository -run '^TestRedeemCampaign' -count=1
CGO_ENABLED=0 go build ./cmd/server
```

集成测试需要本地 Docker。必须确认输出包含实际运行的测试；项目 harness 在 Docker 不可用时会跳过整包，退出码 0 不能作为集成通过的证据。

前端运行相关 `RedeemView`、`admin.redeemCampaign`、i18n 测试以及 `pnpm run build`；浏览器通过临时本地模拟数据验证编辑/选择/生成界面，后端并发和回滚由真实 PostgreSQL 隔离容器验证。
