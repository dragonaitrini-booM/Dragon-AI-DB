from playwright.sync_api import Page, expect

def verify_query_history(page: Page):
    """
    This test verifies that the query history UI is rendered correctly on the Query Builder page.
    """
    # 1. Arrange: Go to the application's login page and log in.
    page.goto("http://localhost:3000/login")
    page.get_by_placeholder("Email").fill("test@example.com")
    page.get_by_placeholder("Password").fill("password")
    page.get_by_role("button", name="Login").click()

    # 2. Act: Navigate to the Query Builder page.
    page.get_by_role("link", name="AI Query").click()

    # 3. Assert: Confirm the Query History component is visible.
    expect(page.get_by_text("Query History")).to_be_visible()

    # 4. Screenshot: Capture the final result for visual verification.
    page.screenshot(path="jules-scratch/verification/query-history-ui.png")
