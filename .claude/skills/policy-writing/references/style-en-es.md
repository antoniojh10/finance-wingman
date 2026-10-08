# Style: English and Spanish

## Both languages
- Sentences under 25 words, one idea each. Active voice, "we" and "you".
- Headings are plain nouns or questions ("What data we keep").
- Tables for retention and vendors; lists for rights.
- Name the real thing: "sign-in link", "workspace", "Railway".
- Explicit numbers and units: "15 minutes", not "a short time".
- Dates spelled out or ISO, never "recently".
- Banned: military-grade, bank-level, unhackable, 100% secure,
  best-in-class, "industry standard" without naming the standard, "may"
  used to hide what we actually do, "such as" lists implying more than
  exists, "GDPR compliant" as a badge.
- Legal terms keep their meaning: controller, processor, subprocessor,
  legal basis.
- Identical structure, heading order and claims in both languages.

## English
- Pick UK or US spelling with the user and keep it.

## Spanish
- Address the reader as "tú" (as in the app UI) unless the user prefers
  "usted"; stay consistent across pages.
- Terms: responsable del tratamiento (controller), encargado del
  tratamiento (processor), subencargado (subprocessor), base jurídica.
  For EU users list rights as acceso, rectificación, supresión,
  limitación, portabilidad y oposición (not "derechos ARCO", which is
  Mexican law).
- Supervisory authority: name the one for the controller's country (for
  Spain, the Agencia Española de Protección de Datos).
- Prefer neutral wording ("quien use el servicio") where it stays readable.
- Align product terms with `apps/web/messages/es.json` so the pages match
  the UI.

## File layout (when producing files)
One Markdown file per document and language, front matter limited to
`title`, `updated` (ISO date) and `lang`, no component imports, so any site
can render them, for example
`content/legal/{privacy,terms,security,subprocessors}.{en,es}.md`
(confirm the path with the ticket).
