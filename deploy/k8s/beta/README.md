# Beta K8s Template

这组模板表达 beta 形态的最小公共结构：

- StatefulSet + headless service
- TLS Secret 挂载位
- ACL Secret 挂载位
- `liveness` / `readiness` / `startup` 探针
- Scylla 作为 cluster delivery backend 的配置入口

它们故意保持云无关，不绑定具体 Ingress、LoadBalancer 或证书签发器。
