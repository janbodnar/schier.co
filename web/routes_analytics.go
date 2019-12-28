package web

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/flosch/pongo2"
	"github.com/gorilla/mux"
	"github.com/gschier/schier.dev/generated/prisma-client"
	"github.com/mileusna/useragent"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

func AnalyticsRoutes(router *mux.Router) {
	router.HandleFunc("/t", routeTrack).Methods(http.MethodGet)
	router.HandleFunc("/analytics/live", routeAnalyticsLive).Methods(http.MethodGet)

	router.HandleFunc("/open", routeAnalytics).Methods(http.MethodGet)
	router.Handle("/analytics", http.RedirectHandler("/open", http.StatusSeeOther)).Methods(http.MethodGet)
}

var analyticsTemplate = pageTemplate("analytics/analytics.html")

func routeAnalyticsLive(w http.ResponseWriter, r *http.Request) {
	client := ctxPrismaClient(r)
	now := time.Now()

	fiveMinutesAgo := now.Add(-time.Minute * 5).Format(time.RFC3339)
	views, err := client.AnalyticsPageViews(&prisma.AnalyticsPageViewsParams{
		Where: &prisma.AnalyticsPageViewWhereInput{
			TimeGte: &fiveMinutesAgo,
		},
	}).Exec(r.Context())
	if err != nil {
		http.Error(w, "Failed to query analytics", http.StatusInternalServerError)
		return
	}

	userMap := make(map[string]int)
	for _, view := range views {
		userMap[view.User] += 1
	}

	w.Header().Set("Content-Type", "application/json")
	j, _ := json.Marshal(struct {
		Live int `json:"count"`
	}{Live: len(userMap)})
	_, _ = w.Write(j)
}

func routeAnalytics(w http.ResponseWriter, r *http.Request) {
	client := ctxPrismaClient(r)

	now := time.Now().UTC()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	start = start.Add(time.Hour * 24)

	dateRange := time.Hour * 24 * 7
	dateBucketSize := time.Hour * 24
	numBuckets := int(dateRange / dateBucketSize)

	// Order from oldest to newest so we can get latest sessions last
	orderBy := prisma.AnalyticsPageViewOrderByInputTimeAsc
	views, err := client.AnalyticsPageViews(&prisma.AnalyticsPageViewsParams{
		OrderBy: &orderBy,
		Where: &prisma.AnalyticsPageViewWhereInput{
			TimeGte: prisma.Str(start.Add(-dateRange).Format(time.RFC3339)),
		},
	}).Exec(r.Context())
	if err != nil {
		http.Error(w, "Failed to query analytics", http.StatusInternalServerError)
		return
	}

	topRefCounters := make(map[string]int)
	topPathCounters := make(map[string]int)
	topPlatformCounters := make(map[string]int)
	topBrowserCounters := make(map[string]int)
	pageViewCounters := make(map[int]int)
	userCounters := make(map[int]map[string]int)
	sessionCounters := make(map[int]map[string]int)

	latestSessionViews := make(map[string]prisma.AnalyticsPageView)

	for _, view := range views {
		t, _ := time.Parse(time.RFC3339, view.Time)
		bucketIndex := int(start.Sub(t) / dateBucketSize)

		userAgent := ua.Parse(view.UserAgent)
		if userAgent.Bot {
			continue
		}

		// Increment simple counters
		pageViewCounters[bucketIndex] += 1
		topPlatformCounters[userAgent.OS] += 1
		topBrowserCounters[userAgent.Name] += 1

		// Increment users
		if _, ok := userCounters[bucketIndex]; !ok {
			userCounters[bucketIndex] = make(map[string]int)
		}
		userCounters[bucketIndex][view.User] += 1

		// Increment sessions
		if _, ok := sessionCounters[bucketIndex]; !ok {
			sessionCounters[bucketIndex] = make(map[string]int)
		}
		sessionCounters[bucketIndex][view.Sess] += 1

		// Add paths
		topPathCounters[view.Path] += 1

		// Add session stuff
		latestSessionViews[view.Sess] = view

		// Increment ref
		if view.Referrer != "" {
			u, err := url.Parse(view.Referrer)
			if err != nil {
				continue
			}

			h := u.Hostname()
			if strings.HasPrefix(h, "schier.") {
				continue
			}

			if strings.Contains(h, "google.") || strings.Contains(h, "googleapis.") {
				h = "Google 🔍"
			}

			if strings.HasSuffix(h, "bing.com") {
				h = "Bing 🔍"
			}

			if h == "duckduckgo.com" {
				h = "DuckDuckGo 🔍"
			}

			if h == "t.co" {
				h = "Twitter"
			}

			topRefCounters[h] += 1
		}
	}

	topPaths := make(counters, 0)
	for path, count := range topPathCounters {
		c := counter{Name: path, Count: float64(count)}
		topPaths = append(topPaths, c)
	}

	topRefs := make(counters, 0)
	for path, count := range topRefCounters {
		c := counter{Name: path, Count: float64(count)}
		topRefs = append(topRefs, c)
	}

	topBrowsers := make(counters, 0)
	for path, count := range topBrowserCounters {
		c := counter{Name: path, Count: float64(count) / float64(len(views))}
		topBrowsers = append(topBrowsers, c)
	}

	topPlatforms := make(counters, 0)
	for path, count := range topPlatformCounters {
		c := counter{Name: path, Count: float64(count) / float64(len(views))}
		topPlatforms = append(topPlatforms, c)
	}

	sort.Sort(topPaths)
	sort.Sort(topRefs)
	sort.Sort(topBrowsers)
	sort.Sort(topPlatforms)

	pageViews := make([]int, numBuckets)
	sessions := make([]int, numBuckets)
	users := make([]int, numBuckets)
	for i := 0; i < numBuckets; i++ {
		pageViews[i] = pageViewCounters[numBuckets-i-1]
		sessions[i] = len(sessionCounters[numBuckets-i-1])
		users[i] = len(userCounters[numBuckets-i-1])
	}

	if len(topPaths) > 50 {
		topPaths = topPaths[0:50]
	}

	if len(topRefs) > 10 {
		topRefs = topRefs[0:10]
	}

	if len(topBrowsers) > 5 {
		topBrowsers = topBrowsers[0:5]
	}

	if len(topPlatforms) > 5 {
		topPlatforms = topPlatforms[0:5]
	}

	totalSessionPages := 0.0
	totalSessionAge := 0.0
	totalSessions := 0.0
	bouncedSessions := 0.0
	for _, v := range latestSessionViews {
		if v.Page == 0 {
			continue
		}

		totalSessions += 1
		totalSessionAge += float64(v.Age)
		totalSessionPages += float64(v.Page)
		if v.Page == 1 {
			bouncedSessions += 1
		}
	}
	avgSessionDuration := totalSessionAge / totalSessions
	avgBounceRate := bouncedSessions / totalSessions
	avgPagesPerSession := totalSessionPages / totalSessions

	subscribers, err := client.Subscribers(&prisma.SubscribersParams{
		Where: &prisma.SubscriberWhereInput{
			Unsubscribed: prisma.Bool(false),
		},
	}).Exec(r.Context())
	if err != nil {
		http.Error(w, "Failed to query subscribers", http.StatusInternalServerError)
		return
	}

	renderTemplate(w, r, analyticsTemplate(), &pongo2.Context{
		"avgSessionDuration": FormatTime(avgSessionDuration),
		"avgBounceRate":      fmt.Sprintf("%.0f%%", avgBounceRate*100),
		"avgPagesPerSession": fmt.Sprintf("%.1f", avgPagesPerSession),
		"pageViews":          pageViews,
		"users":              users,
		"sessions":           sessions,
		"topPaths":           topPaths,
		"topRefs":            topRefs,
		"topPlatforms":       topPlatforms,
		"topBrowsers":        topBrowsers,
		"bucketSizeSeconds":  dateBucketSize / time.Second,
		"numSubscribers":     len(subscribers),
		"pageTitle":          "Analytics",
		"pageDescription":    "Public analytics for schier.co",
	})
}

func routeTrack(w http.ResponseWriter, r *http.Request) {
	// Don't track admins
	if ctxGetLoggedIn(r) {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	userAgent := r.Header.Get("User-Agent")
	parsedUA := ua.Parse(userAgent)

	// Don't track bots
	if parsedUA.Bot {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	ip := r.Header.Get("X-FORWARDED-FOR")
	if ip == "" {
		ip = r.RemoteAddr
	}

	q := r.URL.Query()
	path := q.Get("path")
	prev := q.Get("prev")
	search := q.Get("search")
	ref := q.Get("ref")
	sess := q.Get("sess")
	user := q.Get("user")
	ageStr := q.Get("age")
	pageStr := q.Get("page")

	age, _ := strconv.Atoi(ageStr)
	page, _ := strconv.Atoi(pageStr)

	go func() {
		client := ctxPrismaClient(r)
		_, err := client.CreateAnalyticsPageView(prisma.AnalyticsPageViewCreateInput{
			UserAgent: userAgent,
			Path:      path,
			PPath:     prev,
			Search:    search,
			Referrer:  ref,
			Sess:      sess,
			User:      user,
			Ip:        ip,
			Age:       int32(age),
			Page:      int32(page),
		}).Exec(context.Background())
		if err != nil {
			log.Println("Failed to update analytics", err.Error())
		}
	}()

	w.WriteHeader(http.StatusNoContent)
}

type counter struct {
	Name  string
	Count float64
}

type counters []counter

func (c counters) Len() int {
	return len(c)
}

func (c counters) Less(i, j int) bool {
	if c[i].Count == c[j].Count {
		return c[i].Name < c[j].Name
	}

	return c[i].Count > c[j].Count
}

func (c counters) Swap(i, j int) {
	c[i], c[j] = c[j], c[i]
}
