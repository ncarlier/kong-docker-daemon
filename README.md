# Kong Docker Daemon

This is a sidekick docker daemon used to automatically declare upstreams and
targets into Kong.

## Usage

Start the sidekick container:

```bash
$ docker run -d \
    --volume /var/run/docker.sock:/var/run/docker.sock \
    ncarlier/kong-docker-target
```

Now if you start a container exposing a port and having the label
`kong.upstream`, this container IP and PORT will be declared in Kong as a target
for the labeled upstream.

Example:

```bash
$ docker run -d \
    --label "kong.upstream:sample.foo" \
    infrastructureascode/hello-world
```

## Demo

You can try the demonstration by using the following command:

```bash
$ make deploy logs
```

This command will deploy the following Docker stack:

- A PostgreSQL instance
- A Kong instance using PostgreSQL as database backend
- A Konga instance allowing you to explore and configure Kong with a [GUI](http://localhost:1337)
- A HelloWorld sample API backend with `sample.foo` as upstream name
- And this sidekick daemon

The sidekick daemon should create the upstream `sample.foo` into Kong and add
the container IP and port as an active target of this upstream.
If you play with the status of this sample service you should see impacts onto
the Kong configuration.

Example:

```bash
$ curl -XGET http://localhost:8001/upstreams/sample.foo/targets/active | jq .
$ docker-compose scale sample=2
$ curl -XGET http://localhost:8001/upstreams/sample.foo/targets/active | jq .
$ docker-compose scale sample=1
$ curl -XGET http://localhost:8001/upstreams/sample.foo/targets/active | jq .
$ docker-compose scale sample=0
$ curl -XGET http://localhost:8001/upstreams/sample.foo/targets/active | jq .
$ ...
```


---
