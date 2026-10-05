from pathlib import Path
import base64
from fontTools.ttLib import TTFont
from fontTools.varLib.instancer import instantiateVariableFont
from fontTools.pens.svgPathPen import SVGPathPen
from fontTools.pens.transformPen import TransformPen
root=Path(__file__).resolve().parents[2]
font=TTFont(root/'web/node_modules/@fontsource-variable/unbounded/files/unbounded-latin-wght-normal.woff2')
font=instantiateVariableFont(font,{'wght':650},inplace=False)
glyphs=font.getGlyphSet(); cmap=font.getBestCmap();scale=98/font['head'].unitsPerEm
x=0;parts=[]
for ch in 'mirai':
 name=cmap[ord(ch)];pen=SVGPathPen(glyphs);glyphs[name].draw(TransformPen(pen,(scale,0,0,-scale,x,0)));parts.append(pen.getCommands());x+=font['hmtx'][name][0]*scale
word=' '.join(parts)
logo=base64.b64encode((root/'web/src/assets/mirai-logo.png').read_bytes()).decode()
for theme in ['light','dark']:
 dark=theme=='dark'; ink='#f7f3ed' if dark else '#222633';muted='#c5c9d9' if dark else '#656c7d';colors=['#293447','#403441','#283c48'] if dark else ['#f8d3bb','#f0dce9','#dce9ed'];card='#ffffff0e' if dark else '#ffffff73';line='#ffffff24' if dark else '#ffffffcc'
 svg=f'''<svg xmlns="http://www.w3.org/2000/svg" width="1280" height="360" viewBox="0 0 1280 360" role="img" aria-label="Mirai. Your network, clearly managed.">
<defs><linearGradient id="bg"><stop stop-color="{colors[0]}"/><stop offset=".52" stop-color="{colors[1]}"/><stop offset="1" stop-color="{colors[2]}"/></linearGradient><radialGradient id="glow"><stop stop-color="#ffffff" stop-opacity=".5"/><stop offset="1" stop-color="#ffffff" stop-opacity="0"/></radialGradient></defs>
<rect x="1" y="1" width="1278" height="358" rx="32" fill="url(#bg)" stroke="{line}"/>
<circle cx="210" cy="185" r="180" fill="url(#glow)"/>
<image x="44" y="74" width="235" height="235" href="data:image/png;base64,{logo}"/>
<g fill="{ink}"><text x="319" y="62" font-family="Arial,sans-serif" font-size="12" letter-spacing="3">SELF-HOSTED. BEAUTIFULLY CONNECTED.</text><path d="{word}" transform="translate(314 177)"/></g>
<text x="319" y="222" font-family="Arial,sans-serif" font-size="26" fill="{muted}">Your network. Clearly managed.</text>
<g font-family="Arial,sans-serif" font-size="13" fill="{ink}">
<rect x="318" y="260" width="123" height="35" rx="17" fill="{card}" stroke="{line}"/><text x="338" y="282">Native Linux</text>
<rect x="453" y="260" width="147" height="35" rx="17" fill="{card}" stroke="{line}"/><text x="474" y="282">Telegram built in</text>
<rect x="612" y="260" width="153" height="35" rx="17" fill="{card}" stroke="{line}"/><text x="634" y="282">Signed updates</text></g>
<g fill="none" stroke="{line}" stroke-width="2"><path d="M985 106 C1030 106 1018 220 1078 220 M985 106 C950 106 914 258 935 258"/><circle cx="985" cy="106" r="45" fill="{card}"/><circle cx="1078" cy="220" r="58" fill="{card}"/><circle cx="935" cy="258" r="31" fill="{card}"/></g>
<g stroke="{ink}" stroke-width="2" fill="none" opacity=".7"><rect x="968" y="91" width="34" height="12" rx="4"/><rect x="968" y="110" width="34" height="12" rx="4"/><path d="M1078 197 l20 8 v14 c0 14-20 26-20 26s-20-12-20-26v-14z M1069 218l7 7 12-14"/><circle cx="935" cy="258" r="10"/></g>
<text x="1164" y="323" text-anchor="end" font-family="Arial,sans-serif" font-size="11" letter-spacing="2" fill="{muted}">OWN YOUR INFRASTRUCTURE</text>
</svg>'''
 (root/f'.github/assets/banner-en-{theme}.svg').write_text(svg,encoding='utf-8')
print('Mirai banners generated.')
