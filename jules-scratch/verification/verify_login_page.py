from playwright.sync_api import Page, expect

def verify_login_page(page: Page):
    """
    This test verifies that the login page of the application renders correctly.
    """
    # 1. Arrange: Go to the application's login page.
    page.goto("http://localhost:3000/login")

    # 2. Assert: Confirm the page title is correct.
    expect(page).to_have_title("AI Database Application")

    # 3. Screenshot: Capture the final result for visual verification.
    page.screenshot(path="jules-scratch/verification/login-page.png")
