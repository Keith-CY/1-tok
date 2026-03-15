// Listing search query builder.

export interface SearchParams {
  q?: string;
  category?: string;
  tags?: string[];
  minPrice?: number;
  maxPrice?: number;
  providerOrgId?: string;
}

/**
 * Builds a URLSearchParams from SearchParams.
 */
export function buildSearchQuery(params: SearchParams): URLSearchParams {
  const query = new URLSearchParams();
  if (params.q) query.set("q", params.q);
  if (params.category) query.set("category", params.category);
  if (params.tags) {
    for (const tag of params.tags) {
      query.append("tag", tag);
    }
  }
  if (params.minPrice !== undefined) query.set("minPrice", String(params.minPrice));
  if (params.maxPrice !== undefined) query.set("maxPrice", String(params.maxPrice));
  if (params.providerOrgId) query.set("providerOrgId", params.providerOrgId);
  return query;
}

/**
 * Parses URL search params back into SearchParams.
 */
export function parseSearchQuery(query: URLSearchParams): SearchParams {
  const result: SearchParams = {};
  const q = query.get("q");
  if (q) result.q = q;
  const category = query.get("category");
  if (category) result.category = category;
  const tags = query.getAll("tag");
  if (tags.length > 0) result.tags = tags;
  const minPrice = query.get("minPrice");
  if (minPrice) result.minPrice = Number(minPrice);
  const maxPrice = query.get("maxPrice");
  if (maxPrice) result.maxPrice = Number(maxPrice);
  const providerOrgId = query.get("providerOrgId");
  if (providerOrgId) result.providerOrgId = providerOrgId;
  return result;
}
