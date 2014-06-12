# Vaban

*A quick and easy way to control groups of Varnish Cache hosts using a RESTful JSON API.*

[![Build Status](https://travis-ci.org/martensson/vaban.svg?branch=master)](https://travis-ci.org/martensson/vaban)

This is still an early version but its fully functional and more features are
planned. Currently only supports Varnish 3.x and no authentication. 

Soon to come is support for authentication and both banning both varnish 3 and 4.

## Install Vaban:

**Compile Vaban**

``` sh
go get github.com/ant0ine/go-json-rest/rest
go build vaban.go
```

**Create config.json**

``` json
{
    "group1": {
        "Hosts": [
            "a.example.com:6082",
            "b.example.com:6082",
            "c.example.com:6082"
        ],
        "Version": 3
    },
    "group2":{
        "Hosts": [
            "x.example.com:6082",
            "y.example.com:6082",
            "z.example.com:6082"
        ],
        "Version": 4
    }
}
```

**Make sure that the varnish admin interface is available, usually listening on 0.0.0.0:6082**

**Start Vaban**

``` sh
./vaban
```



## API Examples using curl

#### Get status of Vaban

``` sh
curl -i http://127.0.0.1:3000/
```

#### Get all hosts in group

``` sh
curl -i http://127.0.0.1:3000/v1/service/group1
```

#### Send ping to hosts to see if port is open

``` sh
curl -i http://127.0.0.1:3000/v1/service/group1/ping
```

#### Ban the root of your website.

``` sh
curl -i http://127.0.0.1:3000/v1/service/group1/ban -d '{"Pattern":"/"}'
```

#### Ban all css files

``` sh
curl -i http://127.0.0.1:3000/v1/service/group1/ban -d '{"Pattern":".*css"}'
```
