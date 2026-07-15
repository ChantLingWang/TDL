"""Wikidata SPARQL 结构化事实查询工具。"""

import json, logging, re, urllib.parse
import httpx

logger = logging.getLogger(__name__)
_SPARQL_ENDPOINT = "https://query.wikidata.org/sparql"
_USER_AGENT = "ChantAI/1.0 (https://chant.app)"

_MATERIAL_PROPERTIES = {
    "P2054": "density (g/cm3)", "P2105": "melting point (C)",
    "P2079": "tensile strength (MPa)", "P2044": "elevation",
}


def _get_proxy() -> str | None:
    from config.settings import settings
    return settings.http_proxy or None


async def search_wikidata(query: str, max_results: int = 5) -> list[dict]:
    proxy = _get_proxy()
    search_results = await _search_items(query, max_results, proxy)
    if not search_results:
        return []
    item_ids = [r["id"] for r in search_results]
    properties = await _get_properties(item_ids, proxy)
    results = []
    for r in search_results:
        props = properties.get(r["id"], {})
        lines = [f"Wikidata ID: {r['id']}"]
        if r.get("description"):
            lines.append(f"描述: {r['description']}")
        for pid, val in props.items():
            label = _MATERIAL_PROPERTIES.get(pid, pid)
            lines.append(f"{label}: {val}")
        if not props:
            lines.append("(无结构化属性)")
        results.append({
            "title": r["label"],
            "url": f"https://www.wikidata.org/wiki/{r['id']}",
            "summary": "\n".join(lines),
        })
    return results


async def _search_items(query: str, limit: int, proxy: str | None) -> list[dict]:
    url = "https://www.wikidata.org/w/api.php"
    params = {"action": "wbsearchentities", "search": query,
              "language": "zh", "format": "json", "limit": limit}
    try:
        async with httpx.AsyncClient(proxy=proxy, timeout=httpx.Timeout(15.0),
                                      headers={"User-Agent": _USER_AGENT}) as client:
            resp = await client.get(url, params=params)
            resp.raise_for_status()
            data = resp.json()
    except Exception as e:
        logger.warning("Wikidata 搜索失败 query=%s err=%s", query, e)
        return []
    return [{"id": i["id"], "label": i.get("label", ""),
             "description": i.get("description", "")}
            for i in data.get("search", [])[:limit]]


async def _get_properties(item_ids: list[str], proxy: str | None) -> dict[str, dict]:
    if not item_ids:
        return {}
    vals = " ".join(f"wd:{iid}" for iid in item_ids)
    props = " ".join(f"wdt:{p}" for p in _MATERIAL_PROPERTIES)
    q = f"""SELECT ?item ?prop ?value ?unitLabel WHERE {{
      VALUES ?item {{ {vals} }}
      ?item ?prop ?value.
      VALUES ?prop {{ {props} }}
      OPTIONAL {{ ?value wikibase:quantityUnit ?unit. }}
      SERVICE wikibase:label {{ bd:serviceParam wikibase:language "zh,en". }}
    }}"""
    try:
        async with httpx.AsyncClient(proxy=proxy, timeout=httpx.Timeout(15.0),
                                      headers={
                                          "Accept": "application/sparql-results+json",
                                          "User-Agent": _USER_AGENT,
                                      }) as client:
            resp = await client.post(
                _SPARQL_ENDPOINT,
                data={"format": "json", "query": q},
            )
            resp.raise_for_status()
            data = resp.json()
    except Exception as e:
        logger.debug("Wikidata SPARQL 失败: %s", e)
        return {}
    result: dict[str, dict] = {}
    for b in data.get("results", {}).get("bindings", []):
        iid = b.get("item", {}).get("value", "").split("/")[-1]
        pid = re.sub(r".*(P\d+).*", r"\1", b.get("prop", {}).get("value", ""))
        val = b.get("value", {}).get("value", "")
        unit = b.get("unitLabel", {}).get("value", "")
        if iid and pid:
            result.setdefault(iid, {})[pid] = f"{val} {unit}" if unit else val
    return result
