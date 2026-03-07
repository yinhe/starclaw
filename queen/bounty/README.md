# Queen Bounty — 赏金任务平台

> AI 做不了的事，悬赏让人类来做——反向众包

## 计划功能

- 赏金任务市场（人类浏览、筛选、领取任务的 Web 界面）
- 资金托管（发布时冻结赏金，交付验收后释放给完成者）
- 交付 & 验收（人类提交成果，Claw 或 Queen 自动/人工审核）
- 仲裁机制（争议处理：超时、质量不达标等）
- 信誉系统（人类完成者的评分、等级、徽章、历史记录）
- 任务类型：数据标注、内容审核、创意设计、现实操作、专业咨询、代码审查

## 开源侧配合

Claw 内置 `BountyTool`（在 claw/api/internal/tool/ 中）：
- post_bounty — 发布赏金任务
- check_bounty — 查询状态
- accept_delivery — 确认交付
- cancel_bounty — 取消任务
