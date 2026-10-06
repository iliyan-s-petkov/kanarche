# Pollen levels

The map layer and the area page show three levels per species: low, moderate, high. The bounds are `pollen.species[].levels` in `airbg.yaml`, as `[season start, peak]` in grains/m3.

| level | rule |
|---|---|
| low | below the season start |
| moderate | at or above the season start |
| high | at or above the peak |

| species | season start | peak |
|---|---|---|
| birch, alder, olive, mugwort | 10 | 100 |
| grass, ragweed | 3 | 50 |

## Source

The CAMS/SILAM classification, which rests on clinically relevant thresholds from the European Academy of Allergy and Clinical Immunology (EAACI). EEA Climate-ADAPT states it for the CAMS ground-level pollen forecast:

> For alder, birch, olive and mugwort, concentrations >= 10 pollen/m3 demarcate the pollen season and concentrations >= 100 pollen/m3 demarcate the peak pollen period (Pfaar et al., 2017). For grass and ragweed, concentrations >= 3 pollen/m3 demarcate the pollen season and concentrations >= 50 pollen/m3 demarcate the peak pollen period (Pfaar et al., 2017; 2020).

- https://climate-adapt.eea.europa.eu/en/observatory/publications-data/analysis-data/cams-ground-level-pollen-forecast
- Pfaar O. et al., 2017, Allergy 72(5):713-722, doi 10.1111/all.13092
- Pfaar O. et al., 2020, Allergy 75(5):1099-1106, doi 10.1111/all.14111

The numbers were read from the Climate-ADAPT page; the Pfaar 2017 full text was not available to check them.

## Known limitation

The source defines the thresholds on hourly concentrations. The area page and the map apply them to the daily mean, which is lower than the hourly peak, so a day can read lower here than the hourly scale would put it. The table also shows the daily maximum.

## No Bulgarian scale

No official Bulgarian numeric scale was found. Bulgarian bulletins use four words (low, moderate, high, very high) with no published numbers, so this scale is the European one and is not a national classification.
