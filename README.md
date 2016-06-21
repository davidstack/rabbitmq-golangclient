# rabbitmq-golangclient
项目中使用了rabbitmq 发送详细，想着多个channel共用一个connection，并且具有重连的功能，一直没有找到这样的代码，自己写了个。

介绍：
  1、多个Channel共用一个Connection
  2、Rabbitmq-Server 停止或者重启时，该客户端会不断尝试重新连接
  3、目前只有发送功能，Consume暂时没有实现
