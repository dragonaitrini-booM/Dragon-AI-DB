from playwright.sync_api import Page, expect

def verify_home_page(page: Page):
    """
    This test verifies that the home page of the application renders correctly.
    """
    # 1. Arrange: Go to the application's home page.
    page.goto("http://localhost:3000")

    # 2. Assert: Confirm the page title is correct.
    expect(page).to_have_title("Mi Amor's Quantum Neural Hub")

    # 3. Screenshot: Capture the final result for visual verification.
    page.screenshot(path="jules-scratch/verification/home-page.png")
