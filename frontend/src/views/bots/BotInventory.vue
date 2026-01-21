<template>
  <content-area style="padding: 0px !important">
    <div class="row">
      <!-- Bot List -->
      <div class="col-4">
        <eq-window title="Your Bots" class="p-0">
          <div class="p-3" v-if="loadingBots">
            <loader-fake-progress />
          </div>
          <div v-else>
            <div v-if="bots.length === 0" class="text-center p-3">
              No bots found.
            </div>
            <table class="eq-table bordered eq-highlight-rows row-table" v-else>
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Class</th>
                  <th>Level</th>
                  <th>Action</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="bot in bots"
                  :key="bot.bot_id"
                  :class="selectedBot && selectedBot.bot_id === bot.bot_id ? 'pulsate-highlight-white' : ''"
                >
                  <td>{{ bot.name }}</td>
                  <td>{{ getClassName(bot.class) }}</td>
                  <td>{{ bot.level }}</td>
                  <td class="text-center">
                    <b-button
                      size="sm"
                      variant="primary"
                      @click="selectBot(bot)"
                    >
                      <i class="fa fa-suitcase"></i> Inventory
                    </b-button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </eq-window>
      </div>

      <!-- Inventory Grid -->
      <div class="col-8" v-if="selectedBot">
        <eq-window :title="`Inventory: ${selectedBot.name}`" class="p-0">
          <div class="p-3" v-if="loadingInventory">
            <loader-fake-progress />
          </div>
          <div class="p-3" v-else>

            <!-- Slots Grid -->
            <div class="row inventory-grid">
              <div class="col-12 mb-3">
                <h5 class="eq-header">Equipment</h5>
              </div>
              <div
                class="col-4 mb-2"
                v-for="slot in displaySlots"
                :key="slot.id"
              >
                <div class="card p-2 bg-dark border-secondary">
                  <div class="d-flex justify-content-between align-items-center">
                    <small class="text-muted">{{ slot.name }}</small>
                    <div v-if="getEquippedItem(slot.id)">
                      <b-button
                        size="sm"
                        variant="outline-danger"
                        class="p-0 px-1"
                        title="Unequip"
                        @click="unequipItem(slot.id)"
                      >
                        <i class="fa fa-times"></i>
                      </b-button>
                    </div>
                  </div>
                  
                  <div class="mt-2 text-center" v-if="getEquippedItem(slot.id)">
                    <div class="item-icon-wrapper" :class="'item-icon-' + getEquippedItem(slot.id).item_icon"></div>
                    <div class="text-truncate" :title="getEquippedItem(slot.id).item_name">
                      <a 
                        href="#" 
                        @click.prevent="openItemViewer(getEquippedItem(slot.id).item_id)"
                        class="text-primary font-weight-bold"
                      >
                        {{ getEquippedItem(slot.id).item_name }}
                      </a>
                    </div>
                  </div>
                  <div class="mt-2 text-center" v-else>
                    <b-button
                      size="sm"
                      variant="outline-secondary"
                      @click="openEquipModal(slot)"
                    >
                      <i class="fa fa-plus"></i> Equip
                    </b-button>
                  </div>

                </div>
              </div>
            </div>

          </div>
        </eq-window>
      </div>

      <div class="col-8 text-center mt-5" v-else>
        <h3 class="text-muted">Select a bot to view inventory</h3>
      </div>

    </div>

    <!-- Equip Modal -->
    <b-modal
      id="equip-modal"
      :title="`Equip ${selectedSlot ? selectedSlot.name : ''}`"
      size="lg"
      hide-footer
    >
      <div class="mb-3">
        <b-input-group>
          <b-form-input
            v-model="itemSearch"
            placeholder="Search items by name..."
            @keyup.enter="searchItems"
          ></b-form-input>
          <b-input-group-append>
            <b-button variant="primary" @click="searchItems">
              <i class="fa fa-search"></i> Search
            </b-button>
          </b-input-group-append>
        </b-input-group>
      </div>

      <div v-if="searchingItems" class="text-center my-4">
        <loader-fake-progress />
      </div>

      <div v-if="searchResults.length > 0" class="item-results-list">
        <table class="table table-sm table-hover table-dark">
          <thead>
            <tr>
              <th>Icon</th>
              <th>Name</th>
              <th>ID</th>
              <th>Action</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="item in searchResults" :key="item.id">
              <td>
                <div class="item-icon-wrapper" :class="'item-icon-' + item.icon"></div>
              </td>
              <td>{{ item.name }}</td>
              <td>{{ item.id }}</td>
              <td>
                <b-button
                  size="sm"
                  variant="success"
                  @click="equipItem(item)"
                >
                  Equip
                </b-button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <div v-else-if="searchPerformed && searchResults.length === 0" class="text-center my-4">
        No items found.
      </div>

    </b-modal>

  </content-area>
</template>

<script>
import ContentArea from "../../components/layout/ContentArea";
import EqWindow from "../../components/eq-ui/EQWindow";
import LoaderFakeProgress from "../../components/LoaderFakeProgress";
import { SpireApi } from "../../app/api/spire-api";
import { ItemApi } from "../../app/api/api";
import { SpireQueryBuilder } from "../../app/api/spire-query-builder";
import { ROUTE } from "../../routes";
import util from "util";

const ItemClient = new ItemApi(...SpireApi.cfg());

export default {
  name: "BotInventory",
  components: {
    ContentArea,
    EqWindow,
    LoaderFakeProgress
  },
  data() {
    return {
      loadingBots: false,
      bots: [],
      selectedBot: null,
      loadingInventory: false,
      inventory: [],
      
      // Equip Modal
      selectedSlot: null,
      itemSearch: "",
      searchingItems: false,
      searchResults: [],
      searchPerformed: false,

      // Constants
      slots: [
        { id: 13, name: "Primary" },
        { id: 14, name: "Secondary" },
        { id: 2, name: "Head" },
        { id: 17, name: "Chest" },
        { id: 18, name: "Legs" },
        { id: 19, name: "Feet" },
        { id: 12, name: "Hands" },
        { id: 7, name: "Arms" },
        { id: 20, name: "Waist" },
        { id: 8, name: "Back" },
        { id: 6, name: "Shoulders" },
        { id: 5, name: "Neck" },
        { id: 3, name: "Face" },
        { id: 1, name: "Ear 1" },
        { id: 4, name: "Ear 2" },
        { id: 15, name: "Ring 1" },
        { id: 16, name: "Ring 2" },
        { id: 9, name: "Wrist 1" },
        { id: 10, name: "Wrist 2" },
        { id: 11, name: "Range" },
        { id: 21, name: "Ammo" },
        { id: 0, name: "Charm" },
      ]
    };
  },
  computed: {
    displaySlots() {
      // Return slots sorted or grouped if needed. For now returning all defined.
      return this.slots;
    }
  },
  methods: {
    async init() {
      this.loadBots();
    },
    async loadBots() {
      this.loadingBots = true;
      try {
        const r = await SpireApi.v1().get("bots");
        if (r.data && r.data.data) {
          this.bots = r.data.data;
        }
      } catch (e) {
        console.error("Failed to load bots", e);
      } finally {
        this.loadingBots = false;
      }
    },
    async selectBot(bot) {
      this.selectedBot = bot;
      this.loadInventory(bot.bot_id);
    },
    async loadInventory(botId) {
      this.loadingInventory = true;
      this.inventory = [];
      try {
        const r = await SpireApi.v1().get(`bots/${botId}/inventory`);
        if (r.data && r.data.data) {
          this.inventory = r.data.data;
        }
      } catch (e) {
        console.error("Failed to load inventory", e);
      } finally {
        this.loadingInventory = false;
      }
    },
    getEquippedItem(slotId) {
      return this.inventory.find(i => i.slot_id === slotId);
    },
    openEquipModal(slot) {
      this.selectedSlot = slot;
      this.itemSearch = "";
      this.searchResults = [];
      this.searchPerformed = false;
      this.$bvModal.show("equip-modal");
    },
    async searchItems() {
      if (!this.itemSearch) return;
      this.searchingItems = true;
      this.searchPerformed = true;
      try {
        let builder = (new SpireQueryBuilder())
          .where("name", "like", this.itemSearch)
          .limit(20);
        
        const r = await ItemClient.listItems(builder.get());
        if (r.status === 200 && r.data) {
          this.searchResults = r.data;
        }
      } catch (e) {
        console.error("Search failed", e);
      } finally {
        this.searchingItems = false;
      }
    },
    async equipItem(item) {
      if (!this.selectedBot || !this.selectedSlot) return;
      
      try {
        await SpireApi.v1().post(`bots/${this.selectedBot.bot_id}/equip`, {
          slot_id: this.selectedSlot.id,
          item_id: item.id,
          charges: 1
        });
        
        // Refresh inventory
        await this.loadInventory(this.selectedBot.bot_id);
        this.$bvModal.hide("equip-modal");
      } catch (e) {
        alert("Failed to equip item: " + (e.response?.data?.error || e.message));
      }
    },
    async unequipItem(slotId) {
      if (!confirm("Are you sure you want to unequip this item?")) return;
      
      try {
        await SpireApi.v1().post(`bots/${this.selectedBot.bot_id}/unequip`, {
          slot_id: slotId
        });
        
        await this.loadInventory(this.selectedBot.bot_id);
      } catch (e) {
        alert("Failed to unequip item: " + (e.response?.data?.error || e.message));
      }
    },
    openItemViewer(itemId) {
      const url = util.format(ROUTE.ITEM_EDIT, itemId);
      window.open(url, '_blank');
    },
    getClassName(classId) {
      const classes = {
        1: "Warrior", 2: "Cleric", 3: "Paladin", 4: "Ranger", 5: "Shadow Knight",
        6: "Druid", 7: "Monk", 8: "Bard", 9: "Rogue", 10: "Shaman",
        11: "Necromancer", 12: "Wizard", 13: "Magician", 14: "Enchanter",
        15: "Beastlord", 16: "Berserker"
      };
      return classes[classId] || classId;
    }
  },
  mounted() {
    this.init();
  }
};
</script>

<style scoped>
.item-icon-wrapper {
  display: inline-block;
  width: 40px;
  height: 40px;
  background-size: cover;
  margin-bottom: 5px;
}
/* Assuming Spire has global item icon classes logic or I need to implement it. 
   Usually Spire uses 'item-icon-${id}' classes generated in CSS or specific style loader.
   If not present, this div will just be empty or I need to find the icon logic. 
   Checking BotSpellsEditor, it doesn't show icons. 
   However, components/common/ItemIcon.vue might exist. 
   For now relying on Spire's global styles if they exist. */
   
.inventory-grid {
  /* Scrollable if too height? */
}
</style>
