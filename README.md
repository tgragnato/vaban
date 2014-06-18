# Vaban

*A quick and easy way to control groups of Varnish Cache hosts using a RESTful JSON API.*

[![Build Status](https://travis-ci.org/martensson/vaban.svg?branch=master)](https://travis-ci.org/martensson/vaban)

Vaban is built in Go for super high performance and concurrency. Every request and every ban spawns its own lightweight thread.
It supports Varnish 3 + 4, Authentication, Pattern-based and VCL-based banning.

# Getting Started

### Installing from source

#### Dependencies

* Git
* Go 1.1+

#### Clone and Build locally:

``` sh
git clone https://github.com/martensson/vaban.git
cd vaban
go get github.com/ant0ine/go-json-rest/rest
go build vaban.go
```



#### Create a config.json file:

``` json
{
    "group1": {
        "Hosts": [
            "a.example.com:6082",
            "b.example.com:6082",
            "c.example.com:6082"
        ],
        "Secret": "1111-2222-3333-aaaa-bbbb-cccc"
    },
    "group2":{
        "Hosts": [
            "x.example.com:6082",
            "y.example.com:6082",
            "z.example.com:6082"
        ],
        "Secret": "1111-2222-3333-aaaa-bbbb-cccc"
    }
}
```

#### Running Vaban
``` sh
./vaban -p 4000 -f /path/to/config.json
```


**Make sure that the varnish admin interface is available, listening on 0.0.0.0:6082**


# REST API Reference

**get status**
    GET /

**get all services**
    GET /v1/services

**get all hosts in service**
    GET /v1/service/:service

**tcp scan all hosts**
    GET /v1/service/:service/ping

**ban based on pattern**
    POST /v1/service/:service/ban
    JSON Body: {"Pattern":"..."}

**ban based on vcl**
    POST /v1/service/:service/ban
    JSON Body: {"Vcl":"..."}

# CURL Examples

#### Get status of Vaban

``` sh
curl -i http://127.0.0.1:4000/
```

#### Get all groups

``` sh
curl -i http://127.0.0.1:4000/v1/services
```

#### Get all hosts in group

``` sh
curl -i http://127.0.0.1:4000/v1/service/group1
```

#### Scan hosts to see if tcp port is open

``` sh
curl -i http://127.0.0.1:4000/v1/service/group1/ping
```

#### Ban the root of your website.

``` sh
curl -i http://127.0.0.1:4000/v1/service/group1/ban -d '{"Pattern":"/"}'
```

#### Ban all css files

``` sh
curl -i http://127.0.0.1:4000/v1/service/group1/ban -d '{"Pattern":".*css"}'
```

#### Ban based on VCL, in this case all objects matching a host-header.

``` sh
curl -i http://127.0.0.1:4000/v1/service/group1/ban -d '{"Vcl":"req.http.Host == 'example.com'"}'
```
