# PicShortener API

A high-performance Go microservice designed for dynamic image optimization, resizing, and on-the-fly caching. Built with a strong focus on system reliability, memory-efficient I/O operations, and comprehensive observability.

# Arch

<img width="961" height="432" alt="image" src="https://github.com/user-attachments/assets/6a7c3aba-8c46-4b16-800e-eb222de1c6d9" />

# Technology Stack

* GoLang v1.26.3
* SQL Driver `modernc.org/sqlite`
* YAML Parsing `gopkg.in/yaml.v3`
* Image Processing `golang.org/x/image/draw`
* Monitoring powered by `github.com/prometheus/client_golang/prometheus`

## How to use ✨

Before using, you need to download backend service [here](https://github.com/FlowRamAlltimes/Image-shortener/releases/download/0.3/service)

Configure `config.yml`:
```YAML
server:
  port: ":10000"
  idle: 120s
  read: 10s
  write: 10s
database:
  name: "main.db"
  originals_path: "storage/originals"
  cached_path: "storage/cached"
  temp_file_path: "storage/originals/temp_file.jpg"
```

**GET QUERY** *creates newly optimized photo by your parameters*
```
curl "http://YOUR_IP:10000/images?hash=YOUR_HASH_GIVEN_AFTER_POST_QUERY&width=YOUR_WIDTH&quality=YOUR_QUALITY" --output FILE-NAME.jpg 
```

**POST QUERY** *adds your photo into the server*
```
curl -X POST -F "image=@$HOME/YOUR_PATH_TO_PICTURE" http://YOUR_IP:10000/images 
```

# How to use it on your server? 🫠
### Use scp to run it 24/7 and provide an opportunity to everybody who wants to use this app

```
scp /path/to/local/service user@192.168.1.100:/path/to/remote/folder/
## Set your URL or IP of VPS instead of 192.168.1.100
```

Then just run it in [docker](https://www.docker.com), [systemd](https://systemd.io) or in any other way 🐧
### nohup
```
nohup ./service &
## Logs will be in nohup.out 
```


## Good Luck 🤩
