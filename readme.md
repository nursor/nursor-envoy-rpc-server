goctl rpc protoc proto_file/nursor_rpc.proto --go_out=. --go-grpc_out=. --zrpc_out=.


envoy的rpc，response的header分支处理，返回了错误的header，导致了后边还看不到response的body


## 重构变化

1. 直接对redis的操作，迁移到了[account-manager](https://github.com/nursor/account-manager)这个项目中，这个项目不再直接处理redis；
1. mount bpffs /sys/fs/bpf -t bpf
