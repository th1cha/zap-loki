# zap-loki
zap-loki is a library that integrates with the Zap logging library to send logs directly to a Loki instance via its HTTP API.

### Features
- Batching: Efficiently batches logs to reduce the number of HTTP requests.
- Configurable: Allows customization of batch size, batch wait time, and labels.
- Authentication: Supports Basic and API Key authentication.

### Installation
To install zap-loki, use go get:
```
go get github.com/th1cha/zap-loki
```

### Usage
Here is an example of how to initialize a Zap logger with the zap-loki sink:

```go
package main

import (
	"context"
	"time"
	"go.uber.org/zap"
	"github.com/th1cha/zap-loki"
)

const (
	lokiAddress = "http://localhost:3100"
	appName     = "my-app"
)

func initLogger() (*zap.Logger, error) {
	zapConfig := zap.NewProductionConfig()
	loki := zaploki.New(context.Background(), zaploki.Config{
		Url:          lokiAddress,
		BatchMaxSize: 1000,
		BatchMaxWait: 10 * time.Second,
		Labels:       map[string]string{"app": appName, "instance": "instance-1"},
	})

	return loki.WithCreateLogger(zapConfig)
}

func main() {
	logger, err := initLogger()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	logger.Info("Logger initialized successfully",
		zap.String("user_name", "test"),
		zap.Int("attempt", 1),
		zap.Duration("time", time.Second),
	)
}
```

### Configuration
The zaploki.Config struct allows you to customize the behavior of the logger:

- Url: The URL of your Loki instance.
- BatchMaxSize: Maximum number of log entries to batch before sending to Loki.
- BatchMaxWait: Maximum time to wait before sending a batch of logs.
- Labels: A map of labels to attach to each log entry.
- Auth: Optional. An implementation of the Auth interface for authentication.

### Authentication
You can use Basic or API Key authentication by providing an appropriate authenticator:

##### Basic Authentication

```go
loki := zaploki.New(context.Background(), zaploki.Config{
	Url:          lokiAddress,
	BatchMaxSize: 1000,
	BatchMaxWait: 10 * time.Second,
	Labels:       map[string]string{"app": appName},
	Auth:         &zaploki.BasicAuthenticator{Username: "user", Password: "password"},
})
```

##### API Key Authentication

```go
loki := zaploki.New(context.Background(), zaploki.Config{
	Url:          lokiAddress,
	BatchMaxSize: 1000,
	BatchMaxWait: 10 * time.Second,
	Labels:       map[string]string{"app": appName},
	Auth:         &zaploki.APIKeyAuthenticator{Header: "X-API-Key", APIKey: "my-api-key"},
})
```

### Contributing
Feel free to open issues or pull requests if you have suggestions or improvements. Contributions are always welcome!

### Acknowledgments
This project is forked from [paul-milne/zap-loki](https://github.com/paul-milne/zap-loki). Thanks to paul-milne for the original implementation!