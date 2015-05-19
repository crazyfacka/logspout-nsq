# logspout-nsq
Logspout module to publish logs to a NSQ service

## Usage
The recommended way to build this is to clone the [logspout](https://github.com/gliderlabs/logspout) repository, and to modify the *modules.go* to contain only the following lines:

```go
package main

import (
	_ "github.com/gliderlabs/logspout/httpstream"
	_ "github.com/gliderlabs/logspout/routesapi"
	_ "github.com/crazyfacka/logspout-nsq"
)
```

Issue a simple Docker build command

```bash
$ docker build -t <your_name>/logspout-nsq .
```

Finally you just need to start this new docker container

```bash
docker run --name="logspout" --volume=/var/run/docker.sock:/tmp/docker.sock <your_name>/logspout-nsq "nsq://<nsq_ipaddr>:<nsq_port>?topic=<topic>&svc=<service>&app=<app_name>"
```

## Memo
Later on the idea is to have all this automated in a neat script/docker file/whatevs :)
