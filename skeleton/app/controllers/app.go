package controllers

import "github.com/BSP-Mosaic/revel"

type App struct {
	*revel.Controller
}

func (c App) Index() revel.Result {
	return c.Render()
}
