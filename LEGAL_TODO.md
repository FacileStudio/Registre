# Legal placeholders to fill

The legal texts in `apps/api/modules/legal/content/` are drafts written by an engineer, not a
lawyer. Each starts with a "draft pending legal review" banner and `modules/legal.Draft` is
`true`. Before launch a lawyer must review all sixteen files (four documents, four languages)
and fill in every bracketed placeholder below. Then set `Draft = false` and bump the versions in
`apps/api/internal/consent/consent.go` in the same commit.

| Placeholder | Where | What is needed |
|---|---|---|
| `[SIREN]` | all four documents | Facile Studio's SIREN number |
| `[REGISTERED OFFICE ADDRESS]` | all four | Address of the registered office |
| `[SHARE CAPITAL]` | legal notice | Share capital in euros |
| `[RCS CITY]` | legal notice | City of the RCS registration |
| `[VAT NUMBER]` | CGV, legal notice | Intra-EU VAT number |
| `[DIRECTOR OF PUBLICATION]` | legal notice | Name of the director of publication |
| `[CONTACT EMAIL]`, `[PHONE NUMBER]` | all four | Public contact |
| `[HOSTING PROVIDER ...]` | privacy, legal notice | Name, address and phone of the host (la ruche's provider) |
| `[EMAIL PROVIDER]` | privacy | SMTP provider used by Registre |
| `[CDN OR PROXY PROVIDER, IF ANY]` | privacy | Whether traffic passes through a CDN or proxy, and which |
| `[DPO NAME]` | privacy | Data protection officer, or "none appointed" |
| `[LOG RETENTION]`, `[AUDIT RETENTION]` | privacy | Retention of technical logs (Journal) and of `admin_events` |
| `[LIST TRANSFERS AND SAFEGUARDS]` | privacy | Transfers outside the EEA (Stripe, others) and their legal basis |
| `[MINIMUM AGE]` | CGU | Minimum age; 15 is proposed (French digital majority for consent) |
| `[CONSUMER MEDIATOR ...]` | CGV | The consumer mediator Facile Studio adheres to (mandatory in France) |
| `[NUMBER]` | CGV | Days of failed payment before a space returns to the free plan |

The translated files use the same placeholders in the target language (for example
`[ADRESSE DU SIÈGE]`, `[DIRECCIÓN DEL DOMICILIO SOCIAL]`, `[ANSCHRIFT DES SITZES]`).

Points to check beyond the placeholders:

- The withdrawal-right wording for a digital service that starts at once (Code de la
  consommation L221-28).
- Whether the consent IP digest and its legal basis are acceptable as proof of consent.
- Whether the 30-day erasure promise for apps is achievable for every app and its backups.
