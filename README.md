# Farm Search

<div align="center">

![Cute cartoon sheep](https://cdn-icons-png.flaticon.com/512/3306/3306571.png)

**Discover rural and farm properties for sale in NSW, Australia**

[**View Live Site**](https://farms.dstrek.com)

</div>

---

## About

Farm Search displays properties on an interactive map with filtering by price, property type, land size, and distance from Sydney. Find your perfect rural escape with drive-time isochrones showing how far you can get from the city.

## Features

- Interactive map powered by MapLibre GL JS
- Filter by price range, land size, and property type
- Drive-time isochrones from Sydney CBD
- Distance calculations to nearby towns and schools
- Mobile-friendly responsive design

## Tech Stack

- **Backend**: Go with Chi router and SQLite
- **Frontend**: Vanilla JavaScript with MapLibre GL JS
- **Deployment**: systemd + Caddy with automatic HTTPS

## Development

```bash
# Install dependencies and seed sample data
make setup

# Start development server with live reload
make run

# Deploy to production
make deploy
```

## License

MIT
