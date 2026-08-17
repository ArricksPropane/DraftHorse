#!/usr/bin/env python3
"""ARRICKS-16 — generate every app icon from the canonical brand art.

Source of truth: assets/branding/DraftHorse-1024.png (green horse on a white
rounded plate, transparent corners). Outputs:

  src/app/build/windows/icon.ico   multi-res app icon (16..256) — Wails embeds
                                   it as the exe resource, which Explorer, the
                                   Start Menu shortcut, the go-mapi.mailto
                                   DefaultIcon, and ARP all render. The
                                   installer also ships a copy as
                                   $INSTDIR\go-mapi.ico for toast visuals
                                   (toast_windows.go toastIconPath).
  src/app/build/appicon.png        512px PNG (wails' generic app icon input)
  src/app/assets/tray/*.ico        the four D-16 tray states, drawn from the
                                   horse SILHOUETTE (no white plate — tray
                                   glyphs sit directly on the taskbar):
                                     tray-idle       green horse
                                     tray-has-queue  green horse + amber dot
                                     tray-error      red horse
                                     tray-update     green horse + blue dot
                                   Badge colors keep the pre-rebrand envelope
                                   language (amber=queue, red=error).

Deterministic: same input -> byte-identical outputs (PIL PNG encoding is
stable for a given version; regenerate and commit if PIL changes).

Usage: python3 scripts/generate-icons.py   (from the repo root)
"""

from pathlib import Path

from PIL import Image, ImageDraw

ROOT = Path(__file__).resolve().parent.parent
SRC = ROOT / "assets/branding/DraftHorse-1024.png"

BRAND_GREEN = (16, 73, 42, 255)
# Tray glyphs render at 16px on dark taskbars — the brand green reads as
# near-black there, so the tray tint is the same hue lifted ~20% lightness.
TRAY_GREEN = (23, 102, 59, 255)
ERROR_RED = (200, 44, 34, 255)
QUEUE_AMBER = (255, 179, 0, 255)
UPDATE_BLUE = (25, 118, 210, 255)

APP_ICON_SIZES = [16, 24, 32, 48, 64, 128, 256]
TRAY_SIZES = [16, 32, 48]


def horse_silhouette(src: Image.Image) -> Image.Image:
    """Alpha mask of the horse: dark pixels inside the plate."""
    rgba = src.convert("RGBA")
    out = Image.new("RGBA", rgba.size, (0, 0, 0, 0))
    px = rgba.load()
    op = out.load()
    for y in range(rgba.height):
        for x in range(rgba.width):
            r, g, b, a = px[x, y]
            if a > 128 and (r + g + b) < 380:  # dark = horse, white plate drops out
                op[x, y] = (255, 255, 255, a)
    return out.crop(out.getbbox())


def tint(mask: Image.Image, color: tuple) -> Image.Image:
    solid = Image.new("RGBA", mask.size, color)
    out = Image.new("RGBA", mask.size, (0, 0, 0, 0))
    out.paste(solid, (0, 0), mask)
    return out


def fit_square(im: Image.Image, size: int, pad_ratio: float = 0.04) -> Image.Image:
    """Scale into a size x size transparent square, preserving aspect."""
    pad = int(size * pad_ratio)
    box = size - 2 * pad
    w, h = im.size
    scale = min(box / w, box / h)
    scaled = im.resize((max(1, int(w * scale)), max(1, int(h * scale))), Image.LANCZOS)
    out = Image.new("RGBA", (size, size), (0, 0, 0, 0))
    out.paste(scaled, ((size - scaled.width) // 2, (size - scaled.height) // 2), scaled)
    return out


def badge(im: Image.Image, color: tuple) -> Image.Image:
    """Bottom-right status dot with a 1px-ish transparent ring for contrast."""
    size = im.width
    out = im.copy()
    d = ImageDraw.Draw(out)
    r = max(3, round(size * 0.17))
    cx = size - r - max(1, size // 16)
    cy = size - r - max(1, size // 16)
    ring = max(1, round(size * 0.045))
    d.ellipse([cx - r - ring, cy - r - ring, cx + r + ring, cy + r + ring], fill=(0, 0, 0, 0))
    d.ellipse([cx - r, cy - r, cx + r, cy + r], fill=color)
    return out


def save_ico(path: Path, frames: list) -> None:
    # PIL's ICO writer silently drops any size larger than the BASE image, so
    # the largest frame must lead; the rest ride along via append_images.
    path.parent.mkdir(parents=True, exist_ok=True)
    frames = sorted(frames, key=lambda f: f.width, reverse=True)
    frames[0].save(path, format="ICO", append_images=frames[1:],
                   sizes=[f.size for f in frames])
    # Trust nothing: verify the directory actually contains every size.
    import struct
    data = path.read_bytes()
    count = struct.unpack("<H", data[4:6])[0]
    got = sorted({(data[6 + i * 16] or 256) for i in range(count)})
    want = sorted({f.width for f in frames})
    if got != want:
        raise SystemExit(f"{path}: ICO contains sizes {got}, expected {want}")


def main() -> None:
    src = Image.open(SRC)

    # App icon: the full plated art (reads well at large sizes and in ARP).
    plated = src.convert("RGBA")
    app_frames = [fit_square(plated, s, pad_ratio=0.0) for s in APP_ICON_SIZES]
    save_ico(ROOT / "src/app/build/windows/icon.ico", app_frames)
    fit_square(plated, 512, pad_ratio=0.0).save(ROOT / "src/app/build/appicon.png")

    # Tray states: bare silhouette, tinted, badged.
    mask = horse_silhouette(src)
    states = {
        "tray-idle": (TRAY_GREEN, None),
        "tray-has-queue": (TRAY_GREEN, QUEUE_AMBER),
        "tray-error": (ERROR_RED, None),
        "tray-update": (TRAY_GREEN, UPDATE_BLUE),
    }
    for name, (body, dot) in states.items():
        horse = tint(mask, body)
        frames = []
        for s in TRAY_SIZES:
            frame = fit_square(horse, s)
            if dot is not None:
                frame = badge(frame, dot)
            frames.append(frame)
        save_ico(ROOT / f"src/app/assets/tray/{name}.ico", frames)

    print("generated: build/windows/icon.ico, build/appicon.png, 4 tray icons")


if __name__ == "__main__":
    main()
