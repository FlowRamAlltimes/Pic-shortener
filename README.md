# Image-shortener
This is universal tool which can work on any server. Programm shortcuts your image in any format (.png .jpg .jpeg) and doesn't lose picture's quality. You can append tool's possibilities with beautiful UI client in any browser. It's my first MVP project based on REST API. I hope ypu'll like it 

## How to use 

Before using, you need to download backend service [HERE](https://github.com/FlowRamAlltimes/Image-shortener/releases/download/service-linux_x86_64/0.1)

**GET QUERY**
```bash
curl "http://YOUR_IP:10000/images?hash=YOUR_HASH_GIVEN_AFTER_POST_QUERY&width=YOUR_WIDTH&quality=YOUR_QUALITY" --output FILE-NAME.jpg
```

**POST QUERY**
```bash
curl -X POST -F "image=@$HOME/YOUR_PATH_TO_PICTURE" http://YOUR_IP:10000/images
```
