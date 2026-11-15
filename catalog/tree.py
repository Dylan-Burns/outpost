"""Shelve items into a tree no node of which holds more than LIMIT rows, and
write it as small JSON trees plus packed, gzip-compressed leaves.

An item is a dict with at least "t" (title) and "path" (a tuple of section
labels, possibly empty). Sections come from the source; when a node would
hold more than LIMIT children it is divided, first by initial letter, then
into title ranges whose labels are the shortest prefixes that tell the
neighbouring ranges apart ("Ma – Me").
"""
import gzip
import json
import math
import re
import unicodedata

LIMIT = 200


def norm(s):
    """Sort and search key: case-folded, accents removed, spaces collapsed."""
    s = unicodedata.normalize("NFKD", s)
    s = "".join(c for c in s if not unicodedata.combining(c))
    return re.sub(r"\s+", " ", s.casefold()).strip()


def initial(s):
    k = norm(s)
    k = re.sub(r"^(the|a|an) ", "", k) if False else k
    c = k[:1]
    if not c:
        return "#"
    if "a" <= c <= "z":
        return c.upper()
    if c.isdigit():
        return "0–9"
    if c.isalpha():
        return "Other scripts"
    return "Symbols"


class Node:
    __slots__ = ("label", "children", "items", "desc")

    def __init__(self, label, desc=None):
        self.label = label
        self.children = {}
        self.items = []
        self.desc = desc


def build(items, section_desc=None):
    """Place items by their path, then split oversized nodes. Returns the root Node."""
    root = Node("")
    for it in items:
        n = root
        for label in it["path"]:
            c = n.children.get(label)
            if c is None:
                c = n.children[label] = Node(label, (section_desc or {}).get(label))
            n = c
        n.items.append(it)
    _split(root)
    return root


def _key(x):
    return norm(x.label if isinstance(x, Node) else x["t"])


def _label_of(x):
    return x.label if isinstance(x, Node) else x["t"]


def _cut(s, n):
    return s[:n].rstrip() or s[:n]


def _prefix(a, b):
    """Shortest leading part of a whose key differs from b's leading part."""
    na, nb = norm(a), norm(b)
    i = 1
    while i < min(len(na), 24) and na[:i] == nb[:i]:
        i += 1
    return _cut(a, i) if norm(_cut(a, i)) == na[:i] else na[:i]


def _adopt(node, entries):
    for e in entries:
        if isinstance(e, Node):
            lab = e.label
            while lab in node.children:
                lab += " "
            node.children[lab] = e
        else:
            node.items.append(e)
    return node


def _range_label(group, prev, nxt):
    first, last = group[0], group[-1]
    if isinstance(first, Node) and " – " in first.label:
        a = first.label.split(" – ")[0]
    else:
        a = _prefix(_label_of(first), _label_of(prev)) if prev is not None else _cut(_label_of(first), 1)
    if isinstance(last, Node) and " – " in last.label:
        b = last.label.split(" – ")[-1]
    else:
        b = _prefix(_label_of(last), _label_of(nxt)) if nxt is not None else _cut(_label_of(last), 1)
    if norm(a) == norm(b) and len(group) > 1 and not isinstance(first, Node):
        a, b = _prefix(_label_of(first), _label_of(last)), _prefix(_label_of(last), _label_of(first))
    return a.strip() if norm(a) == norm(b) else f"{a.strip()} – {b.strip()}"


def fit(entries, by_letter=True):
    """Sorted entries grouped so that no list is longer than LIMIT."""
    entries = sorted(entries, key=_key)
    if len(entries) <= LIMIT:
        return entries
    if by_letter:
        letters = {}
        for e in entries:
            letters.setdefault(initial(_label_of(e)), []).append(e)
        if 1 < len(letters) <= LIMIT and len(entries) > 10 * LIMIT:
            order = sorted(letters, key=lambda l: (l in ("0–9", "Symbols", "Other scripts", "#"), l))
            return [_adopt(Node(l), fit(letters[l], False)) for l in order]
    k = math.ceil(len(entries) / LIMIT)
    size = math.ceil(len(entries) / k)
    groups = [entries[i:i + size] for i in range(0, len(entries), size)]
    nodes = []
    for gi, g in enumerate(groups):
        prev = groups[gi - 1][-1] if gi else None
        nxt = groups[gi + 1][0] if gi + 1 < len(groups) else None
        nodes.append(_adopt(Node(_range_label(g, prev, nxt)), g))
    if len(nodes) <= LIMIT:
        return nodes
    return fit_ranges(nodes)


def fit_ranges(nodes):
    """Range nodes grouped into ranges of ranges, keeping their order."""
    while len(nodes) > LIMIT:
        k = math.ceil(len(nodes) / LIMIT)
        size = math.ceil(len(nodes) / k)
        groups = [nodes[i:i + size] for i in range(0, len(nodes), size)]
        nodes = [_adopt(Node(_range_label(g, None, None)), g) for g in groups]
    return nodes


def _split(node):
    for c in node.children.values():
        _split(c)
    rows = len(node.children) + len(node.items)
    if rows <= LIMIT:
        return
    if node.children and node.items:
        # Sections stay sections; the loose items get a section of their own.
        loose = _adopt(Node("Other pages"), fit(node.items))
        node.items = []
        lab = loose.label
        while lab in node.children:
            lab += " "
        node.children[lab] = loose
        if len(node.children) <= LIMIT:
            return
    entries = list(node.children.values()) + node.items
    node.children, node.items = {}, []
    _adopt(node, fit(entries))


def check(node, path=()):
    """Every node holds at most LIMIT rows. Returns the number of items."""
    rows = len(node.children) + len(node.items)
    if rows > LIMIT:
        raise AssertionError(f"{' › '.join(path) or 'root'} has {rows} rows")
    return len(node.items) + sum(check(c, path + (c.label,)) for c in node.children.values())


# --- writing -------------------------------------------------------------------

class PackWriter:
    """Leaves as gzip members appended to files of at most cap bytes."""

    def __init__(self, directory, prefix, cap=20 << 20):
        self.dir, self.prefix, self.cap = directory, prefix, cap
        self.n = -1
        self.f = None
        self.pos = 0
        self.files = []

    def add(self, obj):
        data = json.dumps(obj, ensure_ascii=False, separators=(",", ":")).encode()
        blob = gzip.compress(data, compresslevel=9, mtime=0)
        if self.f is None or self.pos + len(blob) > self.cap:
            if self.f:
                self.f.close()
            self.n += 1
            name = f"{self.prefix}.{self.n}.bin"
            self.files.append(name)
            self.f = open(self.dir / name, "wb")
            self.pos = 0
        off = self.pos
        self.f.write(blob)
        self.pos += len(blob)
        return [self.n, off, len(blob)]

    def close(self):
        if self.f:
            self.f.close()


def row(it):
    return {k: v for k, v in it.items() if k not in ("path", "idx") and v not in (None, "")}


def serialise(node, pack, on_leaf=None, path=()):
    """Tree JSON with each node's own items written to the pack as one leaf.
    on_leaf(path_indices, items) is called for every leaf (for search)."""
    out = {"l": node.label.strip(), "n": 0, "s": 0}
    if node.desc:
        out["d"] = node.desc
    kids = sorted(node.children.values(), key=_child_order)
    if kids:
        out["c"] = []
        for i, c in enumerate(kids):
            sc = serialise(c, pack, on_leaf, path + (i,))
            out["c"].append(sc)
            out["n"] += sc["n"]
            out["s"] += sc["s"]
            out["u"] = out.get("u", 0) + sc.get("u", 0)
    if kids and "ss" in out.get("c", [{}])[0]:
        out["ss"] = [sum(c["ss"][e] for c in out["c"]) for e in range(len(out["c"][0]["ss"]))]
    if node.items:
        out["n"] += len(node.items)
        out["s"] += sum(it.get("s") or 0 for it in node.items)
        if "ss" in node.items[0]:
            own = [sum(max(it["ss"][e] or 0, 0) for it in node.items) for e in range(len(node.items[0]["ss"]))]
            out["ss"] = [a + b for a, b in zip(out.get("ss", [0] * len(own)), own)]
        out["u"] = out.get("u", 0) + sum(1 for it in node.items if it.get("s") is None)
        out["r"] = pack.add([row(it) for it in node.items])
        if on_leaf:
            on_leaf(path, node.items)
    if not out.get("u"):
        out.pop("u", None)
    return out


_TAIL = ("Other pages", "Other scripts", "Symbols", "0–9", "#", "Site pages", "User profiles")


def _child_order(c):
    lab = c.label.strip()
    return (lab in _TAIL, norm(lab))
