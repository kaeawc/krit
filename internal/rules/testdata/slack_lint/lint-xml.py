"""Turns a Lint XML report into sorted `path:line:column: ID: message` lines for the given comma-separated IDs."""
import os
import sys
import xml.etree.ElementTree as ET

report, root, ids = sys.argv[1], os.path.realpath(sys.argv[2]), set(sys.argv[3].split(","))
out = []
for issue in ET.parse(report).getroot().iter("issue"):
    if issue.get("id") not in ids:
        continue
    loc = issue.find("location")
    path = os.path.relpath(os.path.realpath(os.path.join(root, loc.get("file"))), root)
    if path.startswith("src/"):
        path = "src/main/java/" + path[len("src/"):]
    out.append(f"{path}:{loc.get('line')}:{loc.get('column')}: {issue.get('id')}: {issue.get('message')}")
if out:
    print("\n".join(sorted(out)))
