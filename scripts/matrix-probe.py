#!/usr/bin/env python3
"""
Manual matrix probe: draw a specific pattern on the 96x16 matrix via the REST API.

Usage examples:
  matrix-probe.py clear
  matrix-probe.py pixel  0   0  FF0000
  matrix-probe.py line   0   0 95  0  00FF00
  matrix-probe.py line   0   0  0 15  0000FF
  matrix-probe.py tri    0   0 95  0 48 15 FFFF00
  matrix-probe.py rect   0   0 95 15  FFFFFF
  matrix-probe.py poly   x1,y1 x2,y2 ... FF00FF

Coordinate convention (what we SEND the service):
  PNG x = 0..95 (columns, left-to-right in the PNG)
  PNG y = 0..15 (rows, top-to-bottom in the PNG)

What actually shows up on the physical matrix is what we're here to discover.
"""
import base64
import io
import json
import sys
import urllib.request
from PIL import Image, ImageDraw

WIDTH, HEIGHT = 96, 16
ENDPOINT = "http://localhost:8080/display/image"


def hex_to_rgb(s: str) -> tuple:
    s = s.lstrip("#")
    return (int(s[0:2], 16), int(s[2:4], 16), int(s[4:6], 16))


def post_image(img: Image.Image) -> dict:
    buf = io.BytesIO()
    img.save(buf, format="PNG")
    b64 = base64.b64encode(buf.getvalue()).decode("ascii")
    body = json.dumps({
        "image_base64": b64,
        "mode": "static",
        "speed": 1,
        "stay_time": 0,
    }).encode()
    req = urllib.request.Request(ENDPOINT, data=body,
                                 headers={"Content-Type": "application/json"})
    with urllib.request.urlopen(req, timeout=60) as r:
        return json.loads(r.read().decode())


def new_image() -> Image.Image:
    return Image.new("RGB", (WIDTH, HEIGHT), (0, 0, 0))


def cmd_clear() -> Image.Image:
    return new_image()


def cmd_pixel(x: int, y: int, color: tuple) -> Image.Image:
    img = new_image()
    img.putpixel((x, y), color)
    return img


def cmd_line(x1: int, y1: int, x2: int, y2: int, color: tuple) -> Image.Image:
    img = new_image()
    ImageDraw.Draw(img).line([(x1, y1), (x2, y2)], fill=color, width=1)
    return img


def cmd_tri(x1: int, y1: int, x2: int, y2: int, x3: int, y3: int, color: tuple) -> Image.Image:
    img = new_image()
    ImageDraw.Draw(img).polygon([(x1, y1), (x2, y2), (x3, y3)], outline=color)
    return img


def cmd_rect(x1: int, y1: int, x2: int, y2: int, color: tuple) -> Image.Image:
    img = new_image()
    ImageDraw.Draw(img).rectangle([(x1, y1), (x2, y2)], outline=color)
    return img


def cmd_pixels(points_and_color: list) -> Image.Image:
    """Light a specific list of x,y pixels. Last arg is the color."""
    color = hex_to_rgb(points_and_color[-1])
    img = new_image()
    for p in points_and_color[:-1]:
        x, y = map(int, p.split(","))
        img.putpixel((x, y), color)
    return img


def cmd_poly(points_and_color: list) -> Image.Image:
    color = hex_to_rgb(points_and_color[-1])
    pts = [tuple(map(int, p.split(","))) for p in points_and_color[:-1]]
    img = new_image()
    ImageDraw.Draw(img).polygon(pts, outline=color)
    return img


def main():
    if len(sys.argv) < 2:
        print(__doc__, file=sys.stderr)
        sys.exit(2)
    op = sys.argv[1]
    a = sys.argv[2:]

    if op == "clear":
        img = cmd_clear()
    elif op == "pixel":
        img = cmd_pixel(int(a[0]), int(a[1]), hex_to_rgb(a[2]))
    elif op == "line":
        img = cmd_line(int(a[0]), int(a[1]), int(a[2]), int(a[3]), hex_to_rgb(a[4]))
    elif op == "tri":
        img = cmd_tri(int(a[0]), int(a[1]), int(a[2]), int(a[3]),
                      int(a[4]), int(a[5]), hex_to_rgb(a[6]))
    elif op == "rect":
        img = cmd_rect(int(a[0]), int(a[1]), int(a[2]), int(a[3]), hex_to_rgb(a[4]))
    elif op == "pixels":
        img = cmd_pixels(a)
    elif op == "poly":
        img = cmd_poly(a)
    else:
        print(f"unknown op: {op}", file=sys.stderr)
        sys.exit(2)

    resp = post_image(img)
    print(json.dumps(resp))


if __name__ == "__main__":
    main()
