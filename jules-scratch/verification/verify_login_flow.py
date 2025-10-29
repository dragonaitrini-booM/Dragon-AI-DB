from playwright.sync_api import Page, expect

def verify_login_flow(page: Page):
    """
    This test verifies that the login flow of the application works correctly.
    """
    # 1. Arrange: Go to the application's home page.
    page.goto("http://localhost:3000")

    # 2. Act: Log in.
    page.get_by_placeholder("email@love.com").fill("test@example.com")
    page.get_by_placeholder("password").fill("password")
    page.get_by_role("button", name="Enter the Hub of Love").click()

    # 3. Assert: Confirm the login modal is hidden and the main page is visible.
    expect(page.locator("#loginModal")).to_be_hidden()
    expect(page.get_by_text("QUANTUM NEURAL HUB")).to_be_visible()

    # 4. Screenshot: Capture the final result for visual verification.
    page.screenshot(path="jules-scratch/verification/app-home-page.png")
