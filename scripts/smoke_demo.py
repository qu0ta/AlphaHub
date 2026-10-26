"""Exercise real Go -> DS -> PostgreSQL through the frontend proxy.

Creates demo applications. Run against a separate test Compose project, not production.
Python stdlib only. Does not delete or alter existing applications.
"""
import argparse
from datetime import datetime, timedelta, timezone
import json
from pathlib import Path
import urllib.error
import urllib.request

ROOT = Path(__file__).resolve().parents[1]
PASSWORD = "AlphaHubTest2026!"
MODEL_VERSION = "financial_rules_v2"
DEMO_MONTH = "2026-10"
DEMO_PROFILES = {
    "stable": ("client@demo.ru", "7700000016", 200000000, 12, 83, "OFFERED"),
    "medium": ("medium@demo.ru", "7700000023", 100000000, 18, 57, "MANUAL"),
    "risky": ("risky@demo.ru", "7700000030", 500000000, 12, 16, "REJECTED"),
}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base-url", default="http://localhost:5173")
    args = parser.parse_args()
    base = args.base_url.rstrip("/") + "/api/v1"

    def request(method, path, token=None, body=None, expected=200, pdf=None):
        headers = {}
        if token:
            headers["Authorization"] = "Bearer " + token
        if pdf:
            boundary = "AlphaHubSmokeBoundary"
            data = (
                f'--{boundary}\r\nContent-Disposition: form-data; name="document_type"\r\n\r\nFINANCIAL_STATEMENT\r\n'
                f'--{boundary}\r\nContent-Disposition: form-data; name="file"; filename="{pdf.name}"\r\nContent-Type: application/pdf\r\n\r\n'
            ).encode() + pdf.read_bytes() + f"\r\n--{boundary}--\r\n".encode()
            headers["Content-Type"] = "multipart/form-data; boundary=" + boundary
        elif body is not None:
            data = json.dumps(body).encode()
            headers["Content-Type"] = "application/json"
        else:
            data = None
        req = urllib.request.Request(base + path, data=data, headers=headers, method=method)
        try:
            response = urllib.request.urlopen(req, timeout=120)
        except urllib.error.HTTPError as error:
            response = error
        with response:
            content = response.read()
            assert response.status == expected, (method, path, response.status, content.decode(errors="replace"))
            if "application/json" in response.headers.get("Content-Type", ""):
                return json.loads(content)
            return content

    def login(email):
        return request("POST", "/auth/login", body={"email": email, "password": PASSWORD})["access_token"]

    profiles = DEMO_PROFILES
    # Fixed committed PDFs are valid for submissions in October 2026 only.
    # Fail clearly instead of falsifying the server's application date.
    assert datetime.now(timezone.utc).strftime("%Y-%m") == DEMO_MONTH, (
        "Committed PDFs cover April-September 2026. Generate fresh demo PDFs and "
        "update expected profiles before running outside October 2026; see docs/DEMO_DATA.md."
    )
    bank = login("bank@demo.ru")
    clients = {name: login(p[0]) for name, p in profiles.items()}
    products = request("GET", "/products")
    assert {"WORKING_CAPITAL", "equipment_loan"} <= {p["code"] for p in products}
    request("GET", "/bank/applications", clients["stable"], expected=403)
    request("GET", "/applications", expected=401)
    results = []

    def version(app_id, token):
        return request("GET", "/applications/" + app_id, token)["data_version"]

    def submit(profile):
        email, inn, amount, term, score, status = profiles[profile]
        token = clients[profile]
        company = request("GET", "/company/me", token)
        assert company["inn"] == inn
        app = request("POST", "/applications", token, {"product_code": "WORKING_CAPITAL"}, expected=201)
        app_id = app["id"]
        path = "/applications/" + app_id
        app = request("PATCH", path, token, {
            "data_version": app["data_version"], "company_id": company["id"], "requested_amount_kopecks": amount,
            "requested_term_months": term, "purpose": "Автоматическая демо-проверка",
        })
        request("PATCH", path, token, {"data_version": app["data_version"] - 1, "purpose": "stale"}, expected=409)
        doc = request("POST", path + "/documents", token, expected=201,
                      pdf=ROOT / "ds" / "demo_pdfs" / (profile + ".pdf"))
        downloaded = request("GET", path + "/documents/" + doc["id"] + "/download", token)
        assert downloaded.startswith(b"%PDF-")
        finances = request("GET", path + "/financials", token)
        assert len(finances["financials"]) == 6
        request("POST", path + "/financials/confirm", token, expected=400)
        request("POST", path + "/financials/confirm", token, {"data_version": version(app_id, token)})
        app = request("POST", path + "/submit", token)
        scoring = request("GET", path + "/scoring", token)
        assert scoring["model_version"] == MODEL_VERSION, scoring
        assert (app["status"], scoring["score"]) == (status, score), (profile, app, scoring)
        request("GET", path, clients["risky"] if profile != "risky" else clients["stable"], expected=404)
        return app_id, token

    def respond(app_id, token, action):
        path = "/applications/" + app_id
        offer = request("GET", path + "/offers", token)[0]
        result = request("POST", path + "/offers/" + offer["id"] + "/respond", token,
                         {"data_version": version(app_id, token), "action": action})
        assert result["status"] == ("ACCEPTED" if action == "ACCEPT" else "OFFER_DECLINED")
        return result["status"]

    app_id, token = submit("stable")
    assert respond(app_id, token, "ACCEPT") == "ACCEPTED"
    results.append({"scenario": "automatic approval and acceptance", "id": app_id, "score": profiles["stable"][4], "status": "ACCEPTED"})
    app_id, token = submit("stable")
    assert respond(app_id, token, "DECLINE") == "OFFER_DECLINED"
    results.append({"scenario": "client declines", "id": app_id, "status": "OFFER_DECLINED"})
    app_id, token = submit("risky")
    assert request("GET", "/applications/" + app_id + "/offers", token) == []
    results.append({"scenario": "automatic rejection", "id": app_id, "score": profiles["risky"][4], "status": "REJECTED"})

    for action in ["APPROVE", "COUNTER_OFFER", "REJECT", "REQUEST_INFO"]:
        app_id, token = submit("medium")
        path = "/bank/applications/" + app_id
        queue = request("GET", "/bank/applications/page?q=" + app_id, bank)
        assert any(item["id"] == app_id for item in queue["items"])
        request("GET", path, bank)
        request("POST", path + "/claim", bank, {"data_version": version(app_id, token)})
        if action == "REQUEST_INFO":
            info = request("POST", path + "/info-requests", bank, {
                "data_version": version(app_id, token), "description": "Уточните назначение платежей",
                "required_items": ["Пояснение по операциям"],
            }, expected=201)
            assert request("GET", "/applications/" + app_id, token)["status"] == "NEEDS_INFO"
            initial = request("GET", "/applications/" + app_id, token)
            result = request("POST", "/applications/" + app_id + "/info-requests/" + info["id"] + "/respond", token, {
                "data_version": version(app_id, token), "response_comment": "Расчёты с поставщиками",
            })
            assert result["status"] == "MANUAL"
            assert result["submitted_at"] == initial["submitted_at"]
            assert result["annual_rate_bps"] == initial["annual_rate_bps"]
        else:
            body = {"data_version": version(app_id, token), "action": action,
                    "reason_code": "DEMO_REVIEW", "comment": "Проверка сотрудником"}
            if action != "REJECT":
                body["valid_until"] = (datetime.now(timezone.utc) + timedelta(days=7)).isoformat()
            if action == "COUNTER_OFFER":
                body["offer"] = {"amount_kopecks": 40000000, "term_months": 18, "annual_rate_bps": 1990}
            result = request("POST", path + "/decisions", bank, body, expected=201)
            assert result["status"] == ("REJECTED" if action == "REJECT" else "OFFERED")
            if action == "COUNTER_OFFER":
                assert result["offer"]["amount_kopecks"] == 40000000
                respond(app_id, token, "ACCEPT")
        results.append({"scenario": action, "id": app_id, "score": profiles["medium"][4], "status": result["status"]})
    assert request("GET", "/bank/decisions", bank)["total"] >= 3
    assert request("GET", "/bank/archive", bank)["total"] >= 1
    print(json.dumps({"base_url": args.base_url, "passed": results}, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
