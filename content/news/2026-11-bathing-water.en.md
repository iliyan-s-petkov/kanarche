---
title: "Bathing water on the map and where the data comes from"
description: "How Kanarche collects the EEA classes and the samples of RHI Varna, Burgas and Dobrich for bathing sites, and why this data is hard to find."
date: 2026-11-16
type: update
status: published
---

The map has a Bathing water layer. You switch it on from the Layers button. It shows 96 bathing sites in Bulgaria. 92 of them are on the sea and 4 are on lakes and reservoirs. Each site is coloured by its class under the EU Bathing Water Directive.

![The Kanarche map with the Bathing water layer switched on and the panel of a bathing site open](/static/news/bathing-map-en.webp)

## Why this matters

Before we go into the sea, we want to know if the water is clean. The class excellent, good, sufficient or poor is calculated by the European Environment Agency (EEA). It is based on two bacteria, E. coli and intestinal enterococci, over the last four seasons. For the 2025 season, 93 of the sites on the map are excellent, 2 are good and 1 is sufficient.

The class is published once a year. That is why the panel of each site also shows the single samples that the regional health inspectorates (RHI, РЗИ in Bulgarian) take during the season. One sample is not an assessment of the water. It shows what the quality was on the day it was taken.

## How hard the data is to find

The sea samples we show come from three inspectorates. Each publishes them in its own way.

- [RHI Varna](https://rzi-varna.com/?page_id=2635) publishes one PDF file per season. The 2026 file has 24 pages. Each page is a table for one sampling point. Values below the detection limit are written as "под 15" (below 15), and the coordinates are in degrees, minutes and seconds.
- [RHI Burgas](https://rzi-burgas.bg/kachestvo-na-vodite/) publishes a table in the old Excel format (.xls). The files sit in a collapsed section for the results by year.
- [RHI Dobrich](https://rzi-dobrich.egov.bg/wps/portal/rzi.dobrich/main-activities/public-health/swimming-zone) publishes an .xlsx file with a separate sheet for each point. Earlier years are in .rar archives.

![The RHI Varna page with the list of PDF result files by year](/static/news/bathing-rzi-varna-page.webp)

*Source: [RHI Varna, monitoring of sea water in bathing zones](https://rzi-varna.com/?page_id=2635)*

![A page of the RHI Varna PDF for 2026 with the table of samples for one point in Golden Sands](/static/news/bathing-rzi-varna-pdf.webp)

*Source: [RHI Varna, 2026 monitoring results (PDF)](https://www.rzi-varna.com/health/mvodi-2026.pdf)*

There is no common format and no programmatic access. To see the samples for one beach, a person has to know which inspectorate covers it, find the file and look up the point by its number.

## What we built

Kanarche checks the pages of the three inspectorates and downloads the file for the current year. The file is read in an isolated container with no network access. That way a broken or malicious file cannot harm the site. Each point is matched to a site from the EEA list by its number and its coordinates. If something does not match, the file waits for a manual review and nothing from it reaches the map.

![A four step diagram: files from the RHI websites, reading in an isolated container, checks against the EEA sites, the map](/static/news/bathing-pipeline-en.svg)

## What to keep in mind

The bathing season is over. The latest samples on the map are from September 2026. The panel shows the results of the season and says that it has closed.

Kanarche is not an official source. RHI bathing bans are not shown on the map. Before you swim, check the current notices on the site of the relevant RHI.

The Kanarche code is open. You can see how the files are read in the [GitHub repository](https://github.com/iliyan-s-petkov/kanarche).
