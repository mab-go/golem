"""
Generate Claude's Minecraft skin: "The Scholar Who Got Their Hands Dirty"

Design notes:
- Dark blue-gray coat/cloak over a warm brown tunic
- Muted palette: deep navy, slate gray, warm brown, cream
- Alert warm eyes (brown, not glowing)
- Tousled dark hair
- Weathered boots, practical look
- "Curious traveler who stayed" energy

Minecraft skin 64x64 layout (classic 4px arms):

HEAD (8x8x8):
  Top:    (8,0)  to (16,8)
  Bottom: (16,0) to (24,8)
  Right:  (0,8)  to (8,16)
  Front:  (8,8)  to (16,16)
  Left:   (16,8) to (24,16)
  Back:   (24,8) to (32,16)

TORSO (8w x 12h x 4d):
  Top:    (20,16) to (28,20)
  Bottom: (28,16) to (36,20)
  Right:  (16,20) to (20,32)
  Front:  (20,20) to (28,32)
  Left:   (28,20) to (32,32)
  Back:   (32,20) to (40,32)

RIGHT ARM (4w x 12h x 4d):
  Top:    (44,16) to (48,20)
  Bottom: (48,16) to (52,20)
  Right:  (40,20) to (44,32)
  Front:  (44,20) to (48,32)
  Left:   (48,20) to (52,32)
  Back:   (52,20) to (56,32)

RIGHT LEG (4w x 12h x 4d):
  Top:    (4,16) to (8,20)
  Bottom: (8,16) to (12,20)
  Right:  (0,20) to (4,32)
  Front:  (4,20) to (8,32)
  Left:   (8,20) to (12,32)
  Back:   (12,20) to (16,32)

LEFT LEG (1.8+):
  Top:    (20,48) to (24,52)
  Bottom: (24,48) to (28,52)
  Right:  (16,52) to (20,64)
  Front:  (20,52) to (24,64)
  Left:   (24,52) to (28,64)
  Back:   (28,52) to (32,64)

LEFT ARM (1.8+):
  Top:    (36,48) to (40,52)
  Bottom: (40,48) to (44,52)
  Right:  (32,52) to (36,64)
  Front:  (36,52) to (40,64)
  Left:   (40,52) to (44,64)
  Back:   (44,52) to (48,64)

OVERLAY layers (helm, torso L2, arm L2, leg L2) - we'll use
the helm layer for a hood/collar detail.
"""

from pathlib import Path

from PIL import Image

SCRIPT_DIR = Path(__file__).parent

img = Image.new("RGBA", (64, 64), (0, 0, 0, 0))
px = img.load()

# =============================================================================
# COLOR PALETTE
# =============================================================================

# Skin tones
SKIN       = (194, 164, 134)  # warm medium skin
SKIN_SHADE = (172, 142, 114)  # slightly darker for depth
SKIN_DARK  = (152, 124,  98)  # shadow areas

# Hair
HAIR       = ( 48,  38,  32)  # very dark brown, almost black
HAIR_HI    = ( 68,  55,  45)  # slight highlight

# Eyes
EYE_WHITE  = (220, 215, 208)  # off-white
EYE_BROWN  = ( 82,  56,  38)  # warm brown iris
EYE_DARK   = ( 32,  22,  16)  # pupil

# Mouth
MOUTH      = (152, 108,  86)

# Coat (dark navy-blue-gray)
COAT       = ( 38,  44,  56)  # deep blue-gray
COAT_LIGHT = ( 52,  58,  72)  # slightly lighter for folds
COAT_DARK  = ( 28,  32,  42)  # shadow/crease
COAT_EDGE  = ( 22,  26,  36)  # very dark edge

# Tunic/undershirt (warm brown-cream)
TUNIC      = (142, 118,  88)  # warm brown
TUNIC_DARK = (118,  96,  68)  # shadow
TUNIC_LIGHT= (162, 138, 108)  # highlight

# Belt
BELT       = ( 72,  54,  38)  # dark leather brown
BELT_BUCKLE= (168, 148, 102)  # brass/gold accent

# Pants
PANTS      = ( 58,  52,  48)  # dark charcoal brown
PANTS_DARK = ( 42,  38,  34)  # darker shade

# Boots
BOOT       = ( 52,  42,  34)  # dark brown leather
BOOT_DARK  = ( 38,  30,  24)  # sole/shadow
BOOT_HI    = ( 68,  56,  46)  # highlight

# Hood/collar overlay
HOOD       = ( 44,  50,  62)  # slightly lighter than coat
HOOD_DARK  = ( 34,  40,  52)

# =============================================================================
# HELPER: fill a rectangle
# =============================================================================
def fill(x1, y1, x2, y2, color):
    for y in range(y1, y2):
        for x in range(x1, x2):
            px[x, y] = color + (255,) if len(color) == 3 else color

def set_px(x, y, color):
    px[x, y] = color + (255,) if len(color) == 3 else color

# =============================================================================
# HEAD - Front face (8,8) to (16,16) — 8x8 pixels
# =============================================================================

# Row 0-1 (y=8,9): Hair top
for y in [8, 9]:
    for x in range(8, 16):
        set_px(x, y, HAIR)
# Add slight highlight
set_px(10, 8, HAIR_HI)
set_px(13, 8, HAIR_HI)
set_px(9, 9, HAIR_HI)
set_px(14, 9, HAIR_HI)

# Row 2 (y=10): Hair sides, forehead
set_px(8, 10, HAIR)
set_px(9, 10, HAIR)
set_px(10, 10, SKIN)
set_px(11, 10, SKIN)
set_px(12, 10, SKIN)
set_px(13, 10, SKIN)
set_px(14, 10, HAIR)
set_px(15, 10, HAIR)

# Row 3 (y=11): Hair sides, forehead with slight hair fringe
set_px(8, 11, HAIR)
set_px(9, 11, SKIN)
set_px(10, 11, SKIN)
set_px(11, 11, SKIN)
set_px(12, 11, SKIN)
set_px(13, 11, SKIN)
set_px(14, 11, SKIN)
set_px(15, 11, HAIR)

# Row 4 (y=12): Eyes row — symmetric with 2px nose bridge
set_px(8, 12, HAIR)
set_px(9, 12, EYE_WHITE)
set_px(10, 12, EYE_BROWN)
set_px(11, 12, SKIN)
set_px(12, 12, SKIN)
set_px(13, 12, EYE_BROWN)
set_px(14, 12, EYE_WHITE)
set_px(15, 12, HAIR)

# Row 5 (y=13): Under eyes / nose area
set_px(8, 13, HAIR_HI)
set_px(9, 13, SKIN)
set_px(10, 13, SKIN)
set_px(11, 13, SKIN)
set_px(12, 13, SKIN_SHADE)
set_px(13, 13, SKIN)
set_px(14, 13, SKIN)
set_px(15, 13, HAIR_HI)

# Row 6 (y=14): Mouth
set_px(8, 14, SKIN_SHADE)
set_px(9, 14, SKIN)
set_px(10, 14, SKIN)
set_px(11, 14, MOUTH)
set_px(12, 14, MOUTH)
set_px(13, 14, SKIN)
set_px(14, 14, SKIN)
set_px(15, 14, SKIN_SHADE)

# Row 7 (y=15): Chin / jaw
set_px(8, 15, SKIN_DARK)
set_px(9, 15, SKIN_SHADE)
set_px(10, 15, SKIN)
set_px(11, 15, SKIN)
set_px(12, 15, SKIN)
set_px(13, 15, SKIN)
set_px(14, 15, SKIN_SHADE)
set_px(15, 15, SKIN_DARK)

# =============================================================================
# HEAD - Top (8,0)-(16,8): Hair top view
# =============================================================================
fill(8, 0, 16, 8, HAIR)
# Some highlights for texture
set_px(10, 2, HAIR_HI)
set_px(13, 3, HAIR_HI)
set_px(11, 5, HAIR_HI)
set_px(14, 1, HAIR_HI)
set_px(9, 4, HAIR_HI)

# HEAD - Bottom (16,0)-(24,8): underside of head (rarely seen)
fill(16, 0, 24, 8, SKIN_DARK)

# HEAD - Right side (0,8)-(8,16)
fill(0, 8, 8, 10, HAIR)       # top hair
set_px(3, 8, HAIR_HI)
set_px(6, 9, HAIR_HI)
# Row 10-11: hair with forehead window
fill(0, 10, 4, 12, HAIR)
set_px(1, 10, HAIR_HI)
fill(4, 10, 8, 11, SKIN)
fill(4, 11, 8, 12, SKIN)
# Row 12: hair around ear
fill(0, 12, 3, 13, HAIR)
set_px(1, 12, HAIR_HI)
set_px(3, 12, SKIN_SHADE)     # ear
set_px(4, 12, SKIN)
fill(5, 12, 8, 13, SKIN)
# Row 13: hair continues, cheek
fill(0, 13, 3, 14, HAIR)
set_px(2, 13, HAIR_HI)
fill(3, 13, 8, 14, SKIN)
# Row 14: hair tapers
fill(0, 14, 2, 15, HAIR)
set_px(2, 14, SKIN_SHADE)
fill(3, 14, 8, 15, SKIN)
# Row 15: sideburn + jaw
set_px(0, 15, HAIR)
set_px(1, 15, SKIN_DARK)
fill(2, 15, 8, 16, SKIN_SHADE)

# HEAD - Left side (16,8)-(24,16) - mirror of right
fill(16, 8, 24, 10, HAIR)
set_px(20, 8, HAIR_HI)
set_px(17, 9, HAIR_HI)
# Row 10-11: hair with forehead window
fill(20, 10, 24, 12, HAIR)
set_px(22, 10, HAIR_HI)
fill(16, 10, 20, 11, SKIN)
fill(16, 11, 20, 12, SKIN)
# Row 12: hair around ear
fill(21, 12, 24, 13, HAIR)
set_px(22, 12, HAIR_HI)
fill(16, 12, 19, 13, SKIN)
set_px(19, 12, SKIN_SHADE)    # ear
set_px(20, 12, SKIN)
# Row 13: hair continues, cheek
fill(21, 13, 24, 14, HAIR)
set_px(21, 13, HAIR_HI)
fill(16, 13, 21, 14, SKIN)
# Row 14: hair tapers
fill(22, 14, 24, 15, HAIR)
set_px(21, 14, SKIN_SHADE)
fill(16, 14, 21, 15, SKIN)
# Row 15: sideburn + jaw
set_px(23, 15, HAIR)
set_px(22, 15, SKIN_DARK)
fill(16, 15, 22, 16, SKIN_SHADE)

# HEAD - Back (24,8)-(32,16)
fill(24, 8, 32, 10, HAIR)
fill(24, 10, 32, 16, HAIR)
# Some texture variation
set_px(26, 11, HAIR_HI)
set_px(29, 12, HAIR_HI)
set_px(27, 14, HAIR_HI)
set_px(30, 10, HAIR_HI)

# =============================================================================
# TORSO - Front (20,20)-(28,32): 8w x 12h
# Coat over tunic - open coat showing tunic V underneath
# =============================================================================

# Row 0 (y=20): Collar area - coat
for x in range(20, 28):
    set_px(x, 20, COAT_DARK)

# Row 1 (y=21): Coat with tunic showing at V neckline
set_px(20, 21, COAT)
set_px(21, 21, COAT)
set_px(22, 21, COAT_LIGHT)
set_px(23, 21, TUNIC)
set_px(24, 21, TUNIC)
set_px(25, 21, COAT_LIGHT)
set_px(26, 21, COAT)
set_px(27, 21, COAT)

# Row 2 (y=22): Coat opens wider
set_px(20, 22, COAT)
set_px(21, 22, COAT_LIGHT)
set_px(22, 22, TUNIC)
set_px(23, 22, TUNIC)
set_px(24, 22, TUNIC)
set_px(25, 22, TUNIC)
set_px(26, 22, COAT_LIGHT)
set_px(27, 22, COAT)

# Row 3-5 (y=23-25): Coat with tunic center panel
for y in range(23, 26):
    set_px(20, y, COAT)
    set_px(21, y, COAT_LIGHT)
    set_px(22, y, TUNIC)
    set_px(23, y, TUNIC_LIGHT if y == 24 else TUNIC)
    set_px(24, y, TUNIC_LIGHT if y == 24 else TUNIC)
    set_px(25, y, TUNIC)
    set_px(26, y, COAT_LIGHT)
    set_px(27, y, COAT)

# Row 6 (y=26): Belt line
set_px(20, 26, COAT)
set_px(21, 26, BELT)
set_px(22, 26, BELT)
set_px(23, 26, BELT_BUCKLE)
set_px(24, 26, BELT_BUCKLE)
set_px(25, 26, BELT)
set_px(26, 26, BELT)
set_px(27, 26, COAT)

# Row 7-10 (y=27-30): Lower coat
for y in range(27, 31):
    set_px(20, y, COAT_DARK)
    set_px(21, y, COAT)
    set_px(22, y, COAT)
    set_px(23, y, COAT_LIGHT if y == 28 else COAT)
    set_px(24, y, COAT_LIGHT if y == 28 else COAT)
    set_px(25, y, COAT)
    set_px(26, y, COAT)
    set_px(27, y, COAT_DARK)

# Row 11 (y=31): Coat hem
fill(20, 31, 28, 32, COAT_EDGE)

# TORSO - Top (20,16)-(28,20): shoulder view - coat
fill(20, 16, 28, 20, COAT)
# slight collar variation
fill(22, 16, 26, 17, COAT_LIGHT)

# TORSO - Bottom (28,16)-(36,20): underside - coat hem
fill(28, 16, 36, 20, COAT_DARK)

# TORSO - Right side (16,20)-(20,32)
for y in range(20, 32):
    c = COAT_DARK if y == 20 or y == 31 else COAT
    if y == 26:
        c = BELT
    fill(16, y, 20, y+1, c)

# TORSO - Left side (28,20)-(32,32)
for y in range(20, 32):
    c = COAT_DARK if y == 20 or y == 31 else COAT
    if y == 26:
        c = BELT
    fill(28, y, 32, y+1, c)

# TORSO - Back (32,20)-(40,32)
fill(32, 20, 40, 21, COAT_DARK)  # collar
for y in range(21, 26):
    fill(32, y, 40, y+1, COAT)
fill(32, 26, 40, 27, BELT)  # belt
for y in range(27, 31):
    fill(32, y, 40, y+1, COAT)
    # add subtle crease
    set_px(36, y, COAT_LIGHT)
fill(32, 31, 40, 32, COAT_EDGE)

# =============================================================================
# RIGHT ARM (44,20)-(48,32) front: 4w x 12h
# Coat sleeve, hand at bottom
# =============================================================================

# Right arm - Front
for y in range(20, 29):
    c = COAT_DARK if y == 20 else COAT
    if y in (24, 25):
        c = COAT_LIGHT  # fold highlight
    fill(44, y, 48, y+1, c)

# Cuff
fill(44, 29, 48, 30, COAT_EDGE)

# Hands
fill(44, 30, 48, 32, SKIN)
set_px(44, 31, SKIN_SHADE)

# Right arm - Top (44,16)-(48,20)
fill(44, 16, 48, 20, COAT)

# Right arm - Bottom (48,16)-(52,20)
fill(48, 16, 52, 20, SKIN)

# Right arm - Right (40,20)-(44,32)
for y in range(20, 29):
    fill(40, y, 44, y+1, COAT_DARK if y == 20 else COAT)
fill(40, 29, 44, 30, COAT_EDGE)
fill(40, 30, 44, 32, SKIN_SHADE)

# Right arm - Left (48,20)-(52,32)
for y in range(20, 29):
    fill(48, y, 52, y+1, COAT_DARK if y == 20 else COAT)
fill(48, 29, 52, 30, COAT_EDGE)
fill(48, 30, 52, 32, SKIN_SHADE)

# Right arm - Back (52,20)-(56,32)
for y in range(20, 29):
    fill(52, y, 56, y+1, COAT_DARK if y == 20 else COAT)
fill(52, 29, 56, 30, COAT_EDGE)
fill(52, 30, 56, 32, SKIN)

# =============================================================================
# LEFT ARM (36,52)-(40,64) front: mirror of right
# =============================================================================

# Left arm - Front
for y in range(52, 61):
    c = COAT_DARK if y == 52 else COAT
    if y in (56, 57):
        c = COAT_LIGHT
    fill(36, y, 40, y+1, c)
fill(36, 61, 40, 62, COAT_EDGE)
fill(36, 62, 40, 64, SKIN)
set_px(39, 63, SKIN_SHADE)

# Left arm - Top (36,48)-(40,52)
fill(36, 48, 40, 52, COAT)

# Left arm - Bottom (40,48)-(44,52)
fill(40, 48, 44, 52, SKIN)

# Left arm - Right (32,52)-(36,64)
for y in range(52, 61):
    fill(32, y, 36, y+1, COAT_DARK if y == 52 else COAT)
fill(32, 61, 36, 62, COAT_EDGE)
fill(32, 62, 36, 64, SKIN_SHADE)

# Left arm - Left (40,52)-(44,64)
for y in range(52, 61):
    fill(40, y, 44, y+1, COAT_DARK if y == 52 else COAT)
fill(40, 61, 44, 62, COAT_EDGE)
fill(40, 62, 44, 64, SKIN_SHADE)

# Left arm - Back (44,52)-(48,64)
for y in range(52, 61):
    fill(44, y, 48, y+1, COAT_DARK if y == 52 else COAT)
fill(44, 61, 48, 62, COAT_EDGE)
fill(44, 62, 48, 64, SKIN)

# =============================================================================
# RIGHT LEG (4,20)-(8,32) front: 4w x 12h
# Dark pants with boots
# =============================================================================

# Right leg - Front
for y in range(20, 26):
    fill(4, y, 8, y+1, PANTS)
# knee shade
set_px(4, 24, PANTS_DARK)
set_px(5, 25, PANTS_DARK)

# Boot top
fill(4, 26, 8, 27, BOOT_HI)

# Boot
for y in range(27, 31):
    fill(4, y, 8, y+1, BOOT)
# Boot sole
fill(4, 31, 8, 32, BOOT_DARK)
# Boot highlight
set_px(5, 28, BOOT_HI)

# Right leg - Top (4,16)-(8,20)
fill(4, 16, 8, 20, PANTS)

# Right leg - Bottom (8,16)-(12,20) (sole)
fill(8, 16, 12, 20, BOOT_DARK)

# Right leg - Right (0,20)-(4,32)
for y in range(20, 26):
    fill(0, y, 4, y+1, PANTS_DARK)
fill(0, 26, 4, 27, BOOT_HI)
for y in range(27, 31):
    fill(0, y, 4, y+1, BOOT)
fill(0, 31, 4, 32, BOOT_DARK)

# Right leg - Left (8,20)-(12,32)
for y in range(20, 26):
    fill(8, y, 12, y+1, PANTS)
fill(8, 26, 12, 27, BOOT_HI)
for y in range(27, 31):
    fill(8, y, 12, y+1, BOOT)
fill(8, 31, 12, 32, BOOT_DARK)

# Right leg - Back (12,20)-(16,32)
for y in range(20, 26):
    fill(12, y, 16, y+1, PANTS)
fill(12, 26, 16, 27, BOOT_HI)
for y in range(27, 31):
    fill(12, y, 16, y+1, BOOT)
fill(12, 31, 16, 32, BOOT_DARK)

# =============================================================================
# LEFT LEG (20,52)-(24,64) front: mirror of right
# =============================================================================

# Left leg - Front
for y in range(52, 58):
    fill(20, y, 24, y+1, PANTS)
set_px(23, 56, PANTS_DARK)
set_px(22, 57, PANTS_DARK)

fill(20, 58, 24, 59, BOOT_HI)
for y in range(59, 63):
    fill(20, y, 24, y+1, BOOT)
fill(20, 63, 24, 64, BOOT_DARK)
set_px(22, 60, BOOT_HI)

# Left leg - Top (20,48)-(24,52)
fill(20, 48, 24, 52, PANTS)

# Left leg - Bottom (24,48)-(28,52)
fill(24, 48, 28, 52, BOOT_DARK)

# Left leg - Right (16,52)-(20,64)
for y in range(52, 58):
    fill(16, y, 20, y+1, PANTS)
fill(16, 58, 20, 59, BOOT_HI)
for y in range(59, 63):
    fill(16, y, 20, y+1, BOOT)
fill(16, 63, 20, 64, BOOT_DARK)

# Left leg - Left (24,52)-(28,64)
for y in range(52, 58):
    fill(24, y, 28, y+1, PANTS_DARK)
fill(24, 58, 28, 59, BOOT_HI)
for y in range(59, 63):
    fill(24, y, 28, y+1, BOOT)
fill(24, 63, 28, 64, BOOT_DARK)

# Left leg - Back (28,52)-(32,64)
for y in range(52, 58):
    fill(28, y, 32, y+1, PANTS)
fill(28, 58, 32, 59, BOOT_HI)
for y in range(59, 63):
    fill(28, y, 32, y+1, BOOT)
fill(28, 63, 32, 64, BOOT_DARK)

# =============================================================================
# OVERLAY: Helm layer — intentionally left empty (no hood)
# =============================================================================

# =============================================================================
# OVERLAY: Torso Layer 2 (20,36)-(28,48) for coat details
# Add a collar/lapel detail and coat texture
# =============================================================================

# Collar/scarf at top
for x in range(20, 28):
    set_px(x, 36, TUNIC_DARK + (200,))

# Subtle coat texture - just a few accent pixels
set_px(20, 38, COAT_EDGE + (180,))
set_px(27, 38, COAT_EDGE + (180,))
set_px(20, 42, COAT_EDGE + (180,))
set_px(27, 42, COAT_EDGE + (180,))

# =============================================================================
# Save
# =============================================================================

out_path = SCRIPT_DIR / "claude-skin.png"
img.save(out_path, "PNG")

# Upscaled texture atlas preview (16x)
preview = img.resize((64 * 16, 64 * 16), Image.Resampling.NEAREST)
preview_path = SCRIPT_DIR / "claude-skin-preview.png"
preview.save(preview_path, "PNG")

# Assembled front-view render (16x32 native, upscaled 16x)
front = Image.new("RGBA", (16, 32), (0, 0, 0, 0))
front.paste(img.crop(( 8,  8, 16, 16)), ( 4,  0))  # head
front.paste(img.crop((20, 20, 28, 32)), ( 4,  8))  # torso
front.paste(img.crop((44, 20, 48, 32)), ( 0,  8))  # right arm
front.paste(img.crop((36, 52, 40, 64)), (12,  8))  # left arm
front.paste(img.crop(( 4, 20,  8, 32)), ( 4, 20))  # right leg
front.paste(img.crop((20, 52, 24, 64)), ( 8, 20))  # left leg
front_up = front.resize((16 * 16, 32 * 16), Image.Resampling.NEAREST)
front_path = SCRIPT_DIR / "claude-skin-front.png"
front_up.save(front_path, "PNG")

print(f"Skin saved to {out_path}")
print(f"Preview saved to {preview_path}")
print(f"Front render saved to {front_path}")
