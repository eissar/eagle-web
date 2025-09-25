local fidget = require("fidget")

vim.api.nvim_create_autocmd("LspAttach", {
	callback = function(event)
		fidget.notify("setting gopls directory local config")
		local buf = event.buf

		local client = vim.lsp.get_clients({ name = "gopls", bufnr = buf })[1]

		if not client then
			fidget.notify("NONE FOUND")
			return
		end

		-- Update the client config
		if not client.config.settings then
			client.config.settings = {}
		end
		if not client.config.settings.gopls then
			client.config.settings.gopls = {}
		end
		client.config.settings.gopls.buildFlags = { "-tags=dev" }

		-- Notify the client of configuration changes
		vim.lsp.buf_notify(0, "workspace/didChangeConfiguration", {
			settings = { gopls = { buildFlags = { "-tags=dev" } } },
		})
		fidget.notify("gopls settings updated with build flags: -tags=dev", vim.log.levels.INFO)
	end,
})
