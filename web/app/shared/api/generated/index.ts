/**
 * Punto de entrada a los tipos del contrato.
 *
 * `schema.ts` lo genera `openapi-typescript` desde `api/openapi.yaml` y NO SE
 * EDITA. Este archivo tampoco define tipos: solo pone nombres cortos a lo que
 * hay dentro, para que las vistas escriban `Schemas['Setting']` en vez de
 * `components['schemas']['Setting']`.
 *
 * A proposito NO hay un alias por esquema. Una lista escrita a mano habria que
 * mantenerla al dia, y es justo la clase de duplicado que esta HU viene a
 * quitar: si el contrato renombra un esquema, `Schemas['LoQueSea']` deja de
 * comprobar tipos sola, sin que nadie tenga que acordarse de nada.
 *
 * Regenerar: `npm run generate:api` (dentro del contenedor de web; el contrato
 * se monta en /workspace/openapi.yaml).
 */
import type { components, operations, paths } from './schema'

export type Schemas = components['schemas']
export type Operations = operations
export type Paths = paths
