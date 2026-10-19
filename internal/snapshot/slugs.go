package snapshot

// RetiredSlugs maps an area slug that was renamed to its current slug. The
// first transliteration spelled Sofia "sofiya"; those URLs, embeds and API
// paths are in the wild. The one place that list lives.
var RetiredSlugs = map[string]string{
	"sofiya":             "sofia",
	"sofiya-grad-oblast": "sofia-grad-oblast",
}

// CurrentSlug returns the current slug for a retired one, and any other slug unchanged.
func CurrentSlug(slug string) string {
	if cur, ok := RetiredSlugs[slug]; ok {
		return cur
	}
	return slug
}
