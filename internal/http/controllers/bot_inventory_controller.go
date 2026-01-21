package controllers

import (
	"fmt"
	"github.com/EQEmu/spire/internal/database"
	"github.com/EQEmu/spire/internal/http/request"
	"github.com/EQEmu/spire/internal/http/routes"
	"github.com/EQEmu/spire/internal/models"
	"github.com/labstack/echo/v4"
	"github.com/volatiletech/null/v8"
	"net/http"
	"strconv"
)

type BotInventoryController struct {
	db *database.Resolver
}

func NewBotInventoryController(
	db *database.Resolver,
) *BotInventoryController {
	return &BotInventoryController{
		db: db,
	}
}

func (c *BotInventoryController) Routes() []*routes.Route {
	return []*routes.Route{
		routes.RegisterRoute(http.MethodGet, "bots", c.list, nil),
		routes.RegisterRoute(http.MethodGet, "bots/:bot_id/inventory", c.getInventory, nil),
		routes.RegisterRoute(http.MethodPost, "bots/:bot_id/equip", c.equip, nil),
		routes.RegisterRoute(http.MethodPost, "bots/:bot_id/unequip", c.unequip, nil),
	}
}

func (c *BotInventoryController) list(ctx echo.Context) error {
	user := request.GetUser(ctx)
	if user.ID == 0 {
		return ctx.JSON(http.StatusOK, echo.Map{"data": []interface{}{}})
	}

	var bots []models.BotDatum
	// Filter by owner_id explicitly
	err := c.db.Get(models.BotDatum{}, ctx).
		Where("owner_id = ?", user.ID).
		Find(&bots).Error

	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return ctx.JSON(http.StatusOK, echo.Map{"data": bots})
}

type BotInventoryResponse struct {
	models.BotInventory
	ItemName   string `json:"item_name"`
	ItemIcon   int    `json:"item_icon"`
	ItemSlotID int    `json:"item_slot_id"`
}

func (c *BotInventoryController) getInventory(ctx echo.Context) error {
	user := request.GetUser(ctx)
	botID, err := strconv.ParseUint(ctx.Param("bot_id"), 10, 64)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, echo.Map{"error": "Invalid bot ID"})
	}

	// Ownership check
	var bot models.BotDatum
	err = c.db.Get(models.BotDatum{}, ctx).
		Where("bot_id = ? AND owner_id = ?", botID, user.ID).
		First(&bot).Error
	if err != nil {
		return ctx.JSON(http.StatusForbidden, echo.Map{"error": "You do not own this bot"})
	}

	// Fetch inventory with item details
	var inventories []models.BotInventory
	err = c.db.Get(models.BotInventory{}, ctx).
		Where("bot_id = ?", botID).
		Find(&inventories).Error
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	// Fetch item details manually since GORM relationships might not be fully set up for this specific join
	// Collecting item IDs
	itemIDs := make([]uint, 0)
	for _, inv := range inventories {
		if inv.ItemId.Valid {
			itemIDs = append(itemIDs, inv.ItemId.Uint)
		}
	}

	itemsMap := make(map[int]models.Item)
	if len(itemIDs) > 0 {
		var items []models.Item
		// items are in content db
		err = c.db.Get(models.Item{}, ctx).
			Where("id IN ?", itemIDs).
			Find(&items).Error
		if err != nil {
			return ctx.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
		}
		for _, item := range items {
			itemsMap[item.ID] = item
		}
	}

	response := make([]BotInventoryResponse, 0)
	for _, inv := range inventories {
		resp := BotInventoryResponse{
			BotInventory: inv,
		}
		if inv.ItemId.Valid {
			if item, Ok := itemsMap[int(inv.ItemId.Uint)]; Ok {
				resp.ItemName = item.Name
				resp.ItemIcon = item.Icon
				resp.ItemSlotID = item.Slots // bitmask for validation if needed
			}
		}
		response = append(response, resp)
	}

	return ctx.JSON(http.StatusOK, echo.Map{"data": response})
}

type EquipRequest struct {
	SlotID  uint `json:"slot_id"`
	ItemID  uint `json:"item_id"`
	Charges int  `json:"charges"` // Optional
}

func (c *BotInventoryController) equip(ctx echo.Context) error {
	user := request.GetUser(ctx)
	botID, err := strconv.ParseUint(ctx.Param("bot_id"), 10, 64)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, echo.Map{"error": "Invalid bot ID"})
	}

	req := new(EquipRequest)
	if err := ctx.Bind(req); err != nil {
		return ctx.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	// Ownership check
	var bot models.BotDatum
	err = c.db.Get(models.BotDatum{}, ctx).
		Where("bot_id = ? AND owner_id = ?", botID, user.ID).
		First(&bot).Error
	if err != nil {
		return ctx.JSON(http.StatusForbidden, echo.Map{"error": "You do not own this bot"})
	}

	// Validate Item exists
	var item models.Item
	err = c.db.Get(models.Item{}, ctx).First(&item, req.ItemID).Error
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, echo.Map{"error": "Item not found"})
	}

	// Upsert logic (SQL provided in user request)
	// INSERT INTO bot_inventories ... ON DUPLICATE KEY UPDATE ...
	// Using GORM Save/Updates might be tricky with composite keys if not defined perfectly,
	// checking if record exists first is safer or using raw SQL if needed.
	// BotInventory PK is `inventories_index`. We want to unique by bot_id + slot_id.

	var inventory models.BotInventory
	err = c.db.Get(models.BotInventory{}, ctx).
		Where("bot_id = ? AND slot_id = ?", botID, req.SlotID).
		First(&inventory).Error

	if err != nil {
		// Create new
		inventory = models.BotInventory{
			BotId:       uint(botID),
			SlotId:      uint32(req.SlotID),
			ItemId:      null.UintFrom(req.ItemID),
			InstCharges: null.Uint16From(uint16(req.Charges)),
			// Defaults
			InstColor:      0,
			InstNoDrop:     0,
			OrnamentIcon:   0,
			OrnamentIdFile: 0,
		}
		if err := c.db.Get(models.BotInventory{}, ctx).Create(&inventory).Error; err != nil {
			return ctx.JSON(http.StatusInternalServerError, echo.Map{"error": fmt.Sprintf("Failed to equip item: %v", err)})
		}
	} else {
		// Update existing
		inventory.ItemId = null.UintFrom(req.ItemID)
		inventory.InstCharges = null.Uint16From(uint16(req.Charges))
		// Reset augs? User didn't specify. Assuming we keep them unless logic says otherwise.
		// Detailed spec says "upsert row in bot_inventories", standard behavior replaces item.
		// For MVP we just update item_id.
		if err := c.db.Get(models.BotInventory{}, ctx).Save(&inventory).Error; err != nil {
			return ctx.JSON(http.StatusInternalServerError, echo.Map{"error": fmt.Sprintf("Failed to update equipment: %v", err)})
		}
	}

	return ctx.JSON(http.StatusOK, echo.Map{"message": "Item equipped successfully"})
}

type UnequipRequest struct {
	SlotID uint `json:"slot_id"`
}

func (c *BotInventoryController) unequip(ctx echo.Context) error {
	user := request.GetUser(ctx)
	botID, err := strconv.ParseUint(ctx.Param("bot_id"), 10, 64)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, echo.Map{"error": "Invalid bot ID"})
	}

	req := new(UnequipRequest)
	if err := ctx.Bind(req); err != nil {
		return ctx.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	// Ownership check
	var bot models.BotDatum
	err = c.db.Get(models.BotDatum{}, ctx).
		Where("bot_id = ? AND owner_id = ?", botID, user.ID).
		First(&bot).Error
	if err != nil {
		return ctx.JSON(http.StatusForbidden, echo.Map{"error": "You do not own this bot"})
	}

	// Delete
	err = c.db.Get(models.BotInventory{}, ctx).
		Where("bot_id = ? AND slot_id = ?", botID, req.SlotID).
		Delete(&models.BotInventory{}).Error

	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, echo.Map{"error": "Failed to unequip item"})
	}

	return ctx.JSON(http.StatusOK, echo.Map{"message": "Item unequipped successfully"})
}
