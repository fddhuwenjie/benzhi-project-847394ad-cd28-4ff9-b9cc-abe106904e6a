# BENZHI_README

## 项目说明
- 项目：benzhi-project-847394ad-cd28-4ff9-b9cc-abe106904e6a
- 项目用途：完整实现受限空间作业许可闭环 HTTP 服务，覆盖许可草拟与 revision 修订、安全校验提交、两轮审核整改、批准快照、时段激活、退场证据申报、关闭核验、幂等重放和审计时间线，并使用本地 JSON 原子快照持久化。
- Go 工具链：`golang:1.23`
- 前端工具链：无

## 项目描述
- 项目名称：受限空间作业许可闭环服务
- 项目概述：面向厂区安全管理人员的受限空间作业许可 HTTP 服务，将一张许可从作业草拟、风险校验、审核整改、许可生效推进到退场证据核验关闭，并保留可追溯的状态变化记录。项目按 standard 档位规划约 2550 行真实生产 Go 代码和至少 22 个生产 Go 文件。
- 核心工作流：作业负责人草拟许可并登记空间、人员与风险控制措施，系统校验后提交审核；安全审核员可要求整改或批准生效，作业负责人完成作业后提交退场证据，核验员确认隔离恢复、人员撤离和工具清点结果后关闭许可。
- 对外接口：提供版本化 JSON HTTP API，覆盖许可草稿、提交校验、审核整改、生效、退场申报、关闭核验和状态时间线查询；服务支持 -addr=127.0.0.1:<port>，默认监听 127.0.0.1:19081，也支持读取仅含端口号的 PORT 并绑定到对应回环地址，禁止默认绑定 8080、80、3000 或 0.0.0.0。

## 标准构建、运行和测试命令
进入容器后执行：
cd '/app' && GOTOOLCHAIN=local go build ./...
cd /app && GOTOOLCHAIN=local go run ./cmd/server -self-check -addr=127.0.0.1:19081
cd '/app' && GOTOOLCHAIN=local go test ./...

## Docker 构建和进入容器
chmod +x build_benzhi_docker.sh
./build_benzhi_docker.sh benzhi-project-847394ad-cd28-4ff9-b9cc-abe106904e6a-amd64 linux/amd64
./build_benzhi_docker.sh benzhi-project-847394ad-cd28-4ff9-b9cc-abe106904e6a-arm64 linux/arm64
docker run -it benzhi-project-847394ad-cd28-4ff9-b9cc-abe106904e6a-amd64:latest

## 题目验证命令
1. 预期退出码 0：`go test ./...`
2. 预期退出码 0：`go run ./cmd/server -self-check -addr=127.0.0.1:19081`
