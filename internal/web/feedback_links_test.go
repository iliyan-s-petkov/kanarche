package web_test

import (
	"html"
	"os"
	"regexp"
	"strings"
	"testing"

	"kanarche.eu/internal/config"
	"kanarche.eu/internal/i18n"
	"kanarche.eu/internal/snapshot"
	"kanarche.eu/internal/web"
)

const surveyBase = "https://feedback.kanarche.eu/"

func rendererWithFeedback(t *testing.T, surveyURL string) *web.Renderer {
	t.Helper()
	cat, err := i18n.Load()
	if err != nil {
		t.Fatalf("i18n.Load: %v", err)
	}
	cfg := testConfig(t)
	cfg.Feedback = config.Feedback{URL: surveyURL}
	h := snapshot.NewHolder(cfg.Series, config.Wind{})
	h.Store(fixture(t))
	rr, err := web.NewRenderer(cat, h, cfg)
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}
	return rr
}

// surveyAnchors returns every <a> whose href is the survey with the given lang.
func surveyAnchors(body, lang string) []string {
	re := regexp.MustCompile(`<a [^>]*href="https://feedback\.kanarche\.eu/\?lang=` + lang + `"[^>]*>`)
	return re.FindAllString(body, -1)
}

// With the URL unset nothing survey-related renders on any page, so there is no dead link.
func TestFeedbackIsHiddenWhenURLUnset(t *testing.T) {
	rr := rendererWithFeedback(t, "")
	for _, path := range []string{"/", "/en/", "/about", "/en/about", "/about-the-data", "/privacy", "/en/privacy", "/embed"} {
		body := fetch(t, rr, path).Body.String()
		for _, banned := range []string{"feedback.kanarche.eu", "masthead__link--feedback", `id="feedback-h"`, "!nav.feedback"} {
			if strings.Contains(body, banned) {
				t.Errorf("%s: contains %q with the survey URL unset", path, banned)
			}
		}
		text := html.UnescapeString(visibleText(body))
		for _, banned := range []string{"Обратна връзка", "Вашето мнение", "Анкета за обратна връзка", "Your feedback", "Feedback survey"} {
			if strings.Contains(text, banned) {
				t.Errorf("%s: shows %q with the survey URL unset", path, banned)
			}
		}
	}
}

// With the URL set, every link is external, severed from the opener, carries
// the page language and nothing else, and appears the expected number of times.
func TestFeedbackLinksWhenURLSet(t *testing.T) {
	rr := rendererWithFeedback(t, surveyBase)
	for _, tt := range []struct {
		path, lang, other string
		want              int
	}{
		{"/", "bg", "en", 2},
		{"/en/", "en", "bg", 2},
		{"/about", "bg", "en", 3},
		{"/en/about", "en", "bg", 3},
		{"/privacy", "bg", "en", 2},
		{"/en/privacy", "en", "bg", 2},
	} {
		body := fetch(t, rr, tt.path).Body.String()
		anchors := surveyAnchors(body, tt.lang)
		if len(anchors) != tt.want {
			t.Errorf("%s: %d survey anchors with lang=%s, want %d", tt.path, len(anchors), tt.lang, tt.want)
		}
		for _, a := range anchors {
			if !strings.Contains(a, `rel="noopener noreferrer"`) || !strings.Contains(a, `target="_blank"`) {
				t.Errorf("%s: anchor %s lacks rel=\"noopener noreferrer\" or target=\"_blank\"", tt.path, a)
			}
		}
		if n := len(surveyAnchors(body, tt.other)); n != 0 {
			t.Errorf("%s: %d anchors with lang=%s on a %s page", tt.path, n, tt.other, tt.lang)
		}
		if strings.Contains(body, "feedback.kanarche.eu/?lang="+tt.lang+"&") || strings.Count(body, "feedback.kanarche.eu/?") != strings.Count(body, "feedback.kanarche.eu/?lang=") {
			t.Errorf("%s: a survey link carries a parameter other than lang", tt.path)
		}
		if strings.Contains(body, "!nav.feedback") || strings.Contains(body, "!about.feedback") || strings.Contains(body, "!privacy.feedback") {
			t.Errorf("%s: untranslated key marker", tt.path)
		}
	}
	// The embed frame has no site chrome and gets no survey link.
	if body := fetch(t, rr, "/embed").Body.String(); strings.Contains(body, "feedback.kanarche.eu") {
		t.Error("/embed links to the survey")
	}
}

// The approved copy, verbatim, in both languages.
func TestFeedbackCopyIsTheApprovedCopy(t *testing.T) {
	rr := rendererWithFeedback(t, surveyBase)
	for _, tt := range []struct {
		path  string
		wants []string
	}{
		{"/about", []string{"Обратна връзка", "Вашето мнение", "Канарче е нетърговски проект с отворен код. Ако искате да ни кажете какво работи и какво не, има кратка анкета от около 2 минути, без имейл и без проследяване.", "Към анкетата"}},
		{"/en/about", []string{"Feedback", "Your feedback", "Kanarche is a non-commercial open-source project. If you want to tell us what works and what does not, there is a short survey of about 2 minutes, with no email and no tracking.", "Open the survey"}},
		{"/privacy", []string{"Анкета за обратна връзка", "Анкетата е по желание. Тя работи на feedback.kanarche.eu, на сървър в Европейския съюз. Не записваме IP адрес и имейл, а на този поддомейн се използват само сесийни бисквитки, нужни за работата на формата. Отговорите стигат до собственика на проекта по имейл, а резервните копия се пазят 7 дни."}},
		{"/en/privacy", []string{"Feedback survey", "The survey is optional. It runs on feedback.kanarche.eu, on a server in the European Union. We store no IP address and no email, and that subdomain uses only session cookies the form needs to work. Answers reach the project owner by email, and backups are kept for 7 days."}},
	} {
		text := html.UnescapeString(visibleText(fetch(t, rr, tt.path).Body.String()))
		for _, want := range tt.wants {
			if !strings.Contains(text, want) {
				t.Errorf("%s: missing approved copy %q", tt.path, want)
			}
		}
	}
}

// The About section is the first thing on /about, above the existing sections.
func TestFeedbackSectionComesFirstOnAbout(t *testing.T) {
	rr := rendererWithFeedback(t, surveyBase)
	body := fetch(t, rr, "/about").Body.String()
	i, j := strings.Index(body, `id="feedback-h"`), strings.Index(body, `id="values-h"`)
	if i < 0 || j < 0 || i > j {
		t.Errorf("feedback section at %d, values section at %d: want the feedback section first", i, j)
	}
}

// The section is on /about only: /about-the-data must not contain it.
func TestFeedbackSectionIsAbsentFromAboutTheData(t *testing.T) {
	rr := rendererWithFeedback(t, surveyBase)
	for _, path := range []string{"/about-the-data", "/en/about-the-data"} {
		if body := fetch(t, rr, path).Body.String(); strings.Contains(body, `id="feedback-h"`) {
			t.Errorf("%s contains the feedback section, want it only on /about", path)
		}
	}
}

// Phones hide the masthead item and keep the langpick one, like About and News.
func TestFeedbackMastheadItemIsHiddenOnPhones(t *testing.T) {
	css := readFile(t, "static/app.css")
	for _, want := range []string{
		"@media (max-width: 560px) {\n    .masthead__link--feedback { display: none; }",
		".langpick__extra--feedback { display: list-item;",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("app.css lacks the narrow-width rule %q", want)
		}
	}
}

func TestFeedbackSectionLinkGetsTheTouchHitArea(t *testing.T) {
	css := readFile(t, "static/app.css")
	if got := strings.Count(css, ".about-feedback .link"); got != 2 {
		t.Errorf("app.css lists .about-feedback .link %d times in the touch hit-area rules, want 2", got)
	}
}

func readFile(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	return string(b)
}
