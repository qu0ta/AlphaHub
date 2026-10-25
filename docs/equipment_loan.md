# Equipment loan and saved application rate

Migration 000018 adds equipment_loan (500,000–5,000,000 RUB, 12–36 months,
18% per annum, no collateral). Existing products are preserved.

GET /api/v1/products and /products/{code} return annual_rate_bps and
requires_collateral. base_annual_rate_bps remains for existing clients.
POST /api/v1/applications accepts product_code; the server snapshots its
catalog rate in applications.annual_rate_bps. Clients cannot set that rate.
Submission validates the selected product limits. Scoring and re-scoring use
the saved application amount, term, initial submission date and annual rate.
Automatic offers and unchanged manual approvals use the saved rate too;
explicit counter-offers retain their separately specified conditions.

Legacy rows use the last stored scoring request rate where available, otherwise
the catalog rate at migration time. Historical rates absent from snapshots cannot
be recovered. Rollback preserves the product to avoid deleting referenced data.
