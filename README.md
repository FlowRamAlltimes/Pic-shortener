# Image-shortener 📸
This is universal tool which can work on any server. Programm shortcuts your image in any format (.png .jpg .jpeg) and doesn't lose picture's quality. You can append tool's possibilities with beautiful UI client in any browser. It's my first MVP project based on REST API. I hope you'll like it 🌛

<img width="961" height="432" alt="image" src="https://github.com/user-attachments/assets/6a7c3aba-8c46-4b16-800e-eb222de1c6d9" />


## How to use ✨

Before using, you need to download backend service [here](https://github.com/FlowRamAlltimes/Image-shortener/releases/download/0.2/service)

**GET QUERY**
```
curl "http://YOUR_IP:10000/images?hash=YOUR_HASH_GIVEN_AFTER_POST_QUERY&width=YOUR_WIDTH&quality=YOUR_QUALITY" --output FILE-NAME.jpg ## creates new shortcuted photo by your parameters 
```

**POST QUERY** 
```
curl -X POST -F "image=@$HOME/YOUR_PATH_TO_PICTURE" http://YOUR_IP:10000/images ## adds your photo into the server
```

# How to use it on your server? 🫠
### Use scp to run it 24/7 and provide an oppeortunity to everybody who wants to use this app

```
scp /path/to/local/file.txt user@192.168.1.100:/path/to/remote/folder/
## Set your URL or IP of VPS instead of 192.168.1.100
```

Then just run it in [docker](https://www.docker.com), [systemd](https://systemd.io) or in any other way 🐧
### nohup
```
nohup ./service &
## Logs will be in nohup.out 
```

# Can I connect my own client to this backend? 🙃
```
Yeah, absolutely but you need to use "image" as Multipart Form  
```

## Good Luck 🤩