import asyncio
from playwright.async_api import async_playwright

async def main():
    async with async_playwright() as p:
        browser = await p.chromium.launch()
        page = await browser.new_page()

        # 1. Login
        await page.goto("http://localhost:3000")
        await page.locator('input[id="email"]').fill("test@example.com")
        await page.locator('input[id="password"]').fill("password")
        await page.get_by_role("button", name="Enter the Hub of Love").click()
        await page.wait_for_selector("#loginModal", state="hidden")


        # 2. Check for main UI elements
        await page.wait_for_selector(".upload-zone", state="visible")
        await page.wait_for_selector("#queryInput", state="visible")

        # 3. Simulate file upload
        # Click the upload zone to trigger the file input

        # Set the file input
        async with page.expect_file_chooser() as fc_info:
            await page.locator(".upload-zone").click()
        file_chooser = await fc_info.value
        await file_chooser.set_files("test.txt")

        await page.get_by_role("button", name="PROCESS WITH LOVE").click(force=True)


        # 4. Interact with AI
        await page.locator("#queryInput").fill("Analyze the uploaded data")
        await page.get_by_role("button", name="SEND LOVE").click()


        # 5. Capture screenshot
        await page.screenshot(path="user_journey.png")

        await browser.close()

if __name__ == "__main__":
    asyncio.run(main())
