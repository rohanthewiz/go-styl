"""Semantic CSS comparison: go-styl vs stylus output for one sheet.

Usage: cmp.py <go.css> <stylus.css>  (run.sh drives it for all cema sheets)

Each file is reduced to {(at-rule context, selector): {prop: value}} with the
last declaration of a property winning, which is what the cascade sees for
one selector. Values are normalized for formatting-only differences.
"""
import os, re, sys
names = dict(re.findall(r'"([a-z]+)":\s+"(#[0-9a-f]+)"', open(os.path.join(os.path.dirname(__file__), '../../../internal/value/names.go')).read()))

def short(h):
    h = h.lower()
    if len(h) == 7 and h[1]==h[2] and h[3]==h[4] and h[5]==h[6]:
        return '#'+h[1]+h[3]+h[5]
    return h

def normval(v):
    v = re.sub(r'\s+', ' ', v.strip())
    v = re.sub(r'#[0-9a-fA-F]{6}\b|#[0-9a-fA-F]{3}\b', lambda m: short(m.group()), v)
    v = re.sub(r'(?<![\w#-])([a-z]+)(?![\w(-])', lambda m: short(names[m.group(1)]) if m.group(1) in names and m.group(1) != 'transparent' else m.group(1), v)
    v = re.sub(r'(?<![\d.])0+\.(\d)', r'.\1', v)
    v = re.sub(r'\s*,\s*', ',', v)
    v = re.sub(r'\s*/\s*', '/', v)
    v = v.replace("'", '"')
    v = re.sub(r'url\("([^"]*)"\)', r'url(\1)', v)
    v = re.sub(r'\s*!\s*important', ' !important', v)
    return v

def normsel(s):
    s = re.sub(r'\s+', ' ', s.strip())
    s = re.sub(r'\s*([>+~])\s*', r'\1', s)
    return s.replace("'", '"')

def parse(css):
    css = re.sub(r'/\*.*?\*/', '', css, flags=re.S)
    rules = {}
    order = []
    stack = []  # at-rule contexts
    i = 0; n = len(css)
    buf = ''
    def split_sel(t):
        out, depth, cur = [], 0, ''
        for c in t:
            if c in '([': depth += 1
            if c in ')]': depth -= 1
            if c == ',' and depth == 0:
                out.append(cur); cur = ''
            else: cur += c
        out.append(cur)
        return [normsel(x) for x in out if x.strip()]
    while i < n:
        c = css[i]
        if c == '{':
            head = buf.strip(); buf = ''
            if head.startswith('@') and not head.startswith('@font-face') and not head.startswith('@page'):
                stack.append(re.sub(r'\s+', ' ', head)); i += 1; continue
            j = css.index('}', i)
            body = css[i+1:j]
            decls = {}
            for d in re.split(r';(?![^()]*\))', body):
                if ':' not in d: continue
                k, v = d.split(':', 1)
                decls[k.strip().lower()] = normval(v)
            ctx = ' | '.join(stack)
            for s in split_sel(head):
                key = (ctx, s)
                if key not in rules: rules[key] = {}; order.append(key)
                if head.startswith('@font-face'):
                    key = (ctx, s + '#' + str(len(order))); rules[key] = {}; order.append(key)
                rules[key].update(decls)
            i = j + 1; continue
        if c == '}':
            if stack: stack.pop()
            buf = ''; i += 1; continue
        if c == ';' and buf.strip().startswith('@'):
            key = (' | '.join(stack), re.sub(r'\s+',' ',buf.strip()).replace("'",'"'))
            rules.setdefault(key, {}); order.append(key); buf=''; i+=1; continue
        buf += c; i += 1
    return rules

g = parse(open(sys.argv[1]).read())
r = parse(open(sys.argv[2]).read())
diffs = 0
for k in sorted(set(g) | set(r)):
    if k not in g: print('  only stylus:', k, r[k]); diffs += 1; continue
    if k not in r: print('  only go-styl:', k, g[k]); diffs += 1; continue
    for p in sorted(set(g[k]) | set(r[k])):
        if g[k].get(p) != r[k].get(p):
            print(f'  {k[0]} {k[1]} {{ {p}: go={g[k].get(p)!r} stylus={r[k].get(p)!r} }}'); diffs += 1
print(f'{sys.argv[1].split("/")[-1]}: {len(g)} go / {len(r)} stylus selectors, {diffs} diffs')
