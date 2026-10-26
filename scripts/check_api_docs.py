"""Check Go routes/DTOs and runnable HTTP examples against the saved OpenAPI."""
import json
from pathlib import Path
import re

ROOT = Path(__file__).resolve().parents[1]
spec = json.loads((ROOT / "docs/openapi/go.json").read_text(encoding="utf-8"))
main = "\n".join(line for line in (ROOT / "backend/cmd/main.go").read_text(encoding="utf-8").splitlines() if not line.lstrip().startswith("//"))
actual = {(method.lower(), path.removeprefix("/api/v1")) for method, path in re.findall(r'mux.Handle(?:Func)?\(\s*"(GET|POST|PATCH|PUT|DELETE) ([^" ]+)"', main)}
documented = {(method, path) for path, operations in spec["paths"].items() for method in operations if method in {"get", "post", "patch", "put", "delete"}}
errors = []
if actual != documented:
    errors.append(f"Missing routes: {sorted(actual - documented)}; obsolete: {sorted(documented - actual)}")

def properties(schema):
    if "$ref" in schema:
        schema = spec["components"]["schemas"][schema["$ref"].rsplit("/", 1)[1]]
    result = set(schema.get("properties", {}))
    for part in schema.get("allOf", []):
        result.update(properties(part))
    return result

for package in (ROOT / "backend/internal").iterdir():
    model = package / "model.go"
    if not model.is_file():
        continue
    for name, body in re.findall(r"type (\w+) struct \{(.*?)\n\}", model.read_text(encoding="utf-8"), re.S):
        key = package.name + "." + name
        if key not in spec["components"]["schemas"]:
            continue
        fields = {tag.split(",")[0] for tag in re.findall(r'`json:"([^" ]+)"`', body)} - {"-"}
        missing = fields - properties(spec["components"]["schemas"][key])
        if missing:
            errors.append(f"{key}: missing fields {sorted(missing)}")

http = (ROOT / "docs/requests/alphahub.http").read_text(encoding="utf-8")
requests = [(method.lower(), path.split("?", 1)[0]) for method, path in re.findall(r"^(GET|POST|PATCH|PUT|DELETE) \{\{base\}\}([^\s]+)", http, re.M)]
for method, path in actual:
    pattern = re.sub(r"\{[^}]+\}", r"[^/]+", path)
    if not any(m == method and re.fullmatch(pattern, re.sub(r"\{\{([^}]+)\}\}", r"{\1}", p)) for m, p in requests):
        errors.append(f"No .http example: {method.upper()} {path}")
if errors:
    raise SystemExit("\n".join(errors))
print(f"OK: {len(actual)} Go routes documented; DTO fields and HTTP examples covered")
