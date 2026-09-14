# Data licensing, provenance, privacy, and limitations

## Public data and commercial reuse

The application uses public data from the Geodetic Administration of the
Republic of Slovenia (GURS): the Real Estate Market Register (ETN) and the
Register of Spatial Units (RPE). GURS publishes its public geodetic data under
Creative Commons Attribution 4.0 International (CC BY 4.0), permitting sharing,
adaptation, and commercial reuse with attribution.

Every publication or product view must identify:

- **Geodetska uprava Republike Slovenije (GURS)** as the source;
- the data types, specifically ETN and RPE where both are used;
- the applicable data period or collection state;
- that the source data were processed;
- the [CC BY 4.0 licence](https://creativecommons.org/licenses/by/4.0/deed.sl).

GURS does not sponsor, endorse, certify, or accept responsibility for this
application or its derived results. The official GURS terms remain controlling:
<https://www.e-prostor.gov.si/dostopi/javni-dostop/>.

## Provenance shown to users

The optional Google Maps link sends a selected ETN coordinate to Google only
when clicked and warns about approximate/package locations. The property-history
cost/tax scenario runs entirely in the browser: entered prices, dates, expenses
and tax assumptions are neither submitted to the API nor stored. Observed
resale intervals do not identify owners or prove uninterrupted ownership.
See [property history](property-history.md) for evidence and tax limitations.

The interface distinguishes three categories:

1. **Podatek GURS ETN** — a field reported in the public source transaction;
2. **Izračun aplikacije** — a derived statistic, comparison, aggregation, or
   current-market estimate produced by this software;
3. **Statistična modelska ocena** — a prediction produced by a fitted valuation
   model, including its uncertainty interval and reliability evidence.

Aggregates use the active filters and the documented representative population.
The displayed analysis period is not necessarily the archive download date;
the database `source_files` ledger is the authoritative local record of archive
name, dataset year, checksum, and import time.

## Valuation limitations

All estimates and comparisons are informational analytical outputs. They are
not official GURS generalized values, certified appraisal reports, or guarantees
of a realizable sale price or rent. They must not be represented as compliant
professional valuations for secured lending, court proceedings, taxation,
accounting, or another regulated purpose without review and responsibility by
an appropriately qualified professional.

Results can be affected by reporting errors, missing characteristics, bundled
transactions, limited comparable samples, location precision, market changes,
and model error. Validation metrics and intervals describe historical model
performance; they do not eliminate uncertainty for an individual property.

## Personal-data minimization

The inspected public 2025 BUY/KPP and RENT/NP archive schemas contain no buyer,
seller, landlord, tenant, owner, tax-number, or EMŠO fields. The application
does not attempt to enrich transactions with owners or contracting parties.

ETN archives do contain unstructured source-note fields. Although these are not
party fields, free text is unnecessary for the analytics and could theoretically
contain identifying text entered by a reporter. Migration
`027_source_text_minimization.sql` removes those columns from the database, and
the importer deliberately ignores:

- `OPOMBE_O_PRAVNEM_POSLU`;
- `OPOMBE_O_NEPREMICNINI`;
- `OPOMBE_O_ODDANIH_PROSTORIH`.

Application account display names and authenticated anomaly-review notes are
application-owned data, not GURS transaction-party data. They require the
normal privacy, access-control, retention, and deletion policy of the eventual
commercial service.

Exact addresses, transaction dates, and prices can still contribute to
re-identification when combined with external information. The product must
not add owner names, party identities, behavioural profiles, or external
identity enrichment without a separate documented lawful basis and privacy
assessment.
