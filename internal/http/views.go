package http

import (
	"fmt"
	"html/template"
	"io/fs"
	"math"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"pellets-tracker/internal/core"
	"pellets-tracker/web"
)

type pageData struct {
	Title     string
	ActiveNav string
	Flash     *flashMessage
	Data      any
}

type flashMessage struct {
	Kind    string
	Message string
}

type purchaseView struct {
	core.Purchase
	BrandName string
}

type homeView struct {
	Purchases     []purchaseView
	Brands        []core.Brand
	TotalInvested core.Money
}

type brandsView struct {
	Brands []core.Brand
}

type consumptionView struct {
	core.Consumption
	BrandName string
}

type consumptionsView struct {
	Consumptions []consumptionView
	Brands       []core.Brand
}

type monthlyPoint struct {
	Label         string
	Bags          int
	HeightPercent int
}

type consumptionDetail struct {
	core.ConsumptionCost
	BrandName string
}

type statsView struct {
	Invested  core.Money
	Consumed  core.Money
	Average   core.Money
	Inventory core.InventorySummary
	Monthly   []monthlyPoint
	Details   []consumptionDetail
}

var (
	templateOnce sync.Once
	templates    map[string]*template.Template
	staticOnce   sync.Once
	staticSrv    http.Handler
)

func newTemplateSet() map[string]*template.Template {
	templateOnce.Do(func() {
		pages := map[string]string{
			"home":         "templates/home.tmpl",
			"brands":       "templates/brands.tmpl",
			"consumptions": "templates/consumptions.tmpl",
			"stats":        "templates/stats.tmpl",
		}
		templates = make(map[string]*template.Template, len(pages))
		for name, file := range pages {
			tmpl := template.Must(template.New(name).Funcs(templateFuncMap()).ParseFS(web.Assets, "templates/layout.tmpl", file))
			templates[name] = tmpl
		}
	})
	return templates
}

func staticFileServer() http.Handler {
	staticOnce.Do(func() {
		fsys, err := fs.Sub(web.Assets, "static")
		if err != nil {
			panic(fmt.Sprintf("load static assets: %v", err))
		}
		staticSrv = http.FileServer(http.FS(fsys))
	})
	return staticSrv
}

func templateFuncMap() template.FuncMap {
	return template.FuncMap{
		"formatMoney": func(m core.Money) string { return core.FormatMoney(m) },
		"formatDate": func(t time.Time) string {
			if t.IsZero() {
				return ""
			}
			return t.Format("02/01/2006")
		},
		"formatWeight": func(v float64) string {
			s := fmt.Sprintf("%.2f", v)
			return strings.ReplaceAll(s, ".", ",")
		},
	}
}

func newHomeView(ds *core.DataStore) homeView {
	brands := append([]core.Brand(nil), ds.Brands...)
	sort.Slice(brands, func(i, j int) bool { return strings.ToLower(brands[i].Name) < strings.ToLower(brands[j].Name) })
	purchases := append([]core.Purchase(nil), ds.Purchases...)
	sort.Slice(purchases, func(i, j int) bool { return purchases[i].PurchasedAt.After(purchases[j].PurchasedAt) })
	lookup := brandLookup(ds.Brands)
	rows := make([]purchaseView, len(purchases))
	for i, p := range purchases {
		rows[i] = purchaseView{Purchase: p, BrandName: lookup[p.BrandID]}
	}
	total := core.ComputeInvesti(ds, time.Time{}, time.Time{})
	return homeView{Purchases: rows, Brands: brands, TotalInvested: total}
}

func newBrandsView(ds *core.DataStore) brandsView {
	brands := append([]core.Brand(nil), ds.Brands...)
	sort.Slice(brands, func(i, j int) bool { return strings.ToLower(brands[i].Name) < strings.ToLower(brands[j].Name) })
	return brandsView{Brands: brands}
}

func newConsumptionsView(ds *core.DataStore) consumptionsView {
	brands := append([]core.Brand(nil), ds.Brands...)
	sort.Slice(brands, func(i, j int) bool { return strings.ToLower(brands[i].Name) < strings.ToLower(brands[j].Name) })
	consumptions := append([]core.Consumption(nil), ds.Consumptions...)
	sort.Slice(consumptions, func(i, j int) bool { return consumptions[i].ConsumedAt.After(consumptions[j].ConsumedAt) })
	lookup := brandLookup(ds.Brands)
	rows := make([]consumptionView, len(consumptions))
	for i, c := range consumptions {
		rows[i] = consumptionView{Consumption: c, BrandName: lookup[c.BrandID]}
	}
	return consumptionsView{Consumptions: rows, Brands: brands}
}

func newStatsView(ds *core.DataStore, invested, consumed, average core.Money, monthly []core.MonthlyBags, inventory core.InventorySummary, details []core.ConsumptionCost) statsView {
	lookup := brandLookup(ds.Brands)
	inv := inventory
	sort.Slice(inv.Brands, func(i, j int) bool {
		return strings.ToLower(inv.Brands[i].BrandName) < strings.ToLower(inv.Brands[j].BrandName)
	})
	points := make([]monthlyPoint, 0, len(monthly))
	maxBags := 0
	for _, m := range monthly {
		if m.Bags > maxBags {
			maxBags = m.Bags
		}
	}
	for _, m := range monthly {
		height := 0
		if maxBags > 0 {
			height = int(math.Round(float64(m.Bags) / float64(maxBags) * 100))
			if height < 12 && m.Bags > 0 {
				height = 12
			}
		}
		points = append(points, monthlyPoint{Label: formatMonthLabel(m.Month), Bags: m.Bags, HeightPercent: height})
	}
	detailsView := make([]consumptionDetail, len(details))
	for i, d := range details {
		detailsView[i] = consumptionDetail{ConsumptionCost: d, BrandName: lookup[d.Consumption.BrandID]}
	}
	return statsView{Invested: invested, Consumed: consumed, Average: average, Inventory: inv, Monthly: points, Details: detailsView}
}

func formatMonthLabel(t time.Time) string {
	months := []string{"Jan.", "Fév.", "Mars", "Avr.", "Mai", "Juin", "Juil.", "Août", "Sept.", "Oct.", "Nov.", "Déc."}
	month := months[int(t.Month())-1]
	return fmt.Sprintf("%s %d", month, t.Year())
}

func brandLookup(brands []core.Brand) map[core.ID]string {
	lookup := make(map[core.ID]string, len(brands))
	for _, b := range brands {
		lookup[b.ID] = b.Name
	}
	return lookup
}
