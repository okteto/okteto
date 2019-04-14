import React, { Component } from 'react';
import * as ReactRedux from 'react-redux';
import PropTypes from 'prop-types';
import autobind from 'autobind-decorator';

import Button from 'components/Button';
import Modal from 'components/Modal';
import { createDatabase } from 'actions/databases';

import './CreateDatabase.scss';

class CreateDatabase extends Component {
  constructor(props) {
    super(props);

    this.state = {
      name: ''
    };
  }

  @autobind
  handleConfirmClick() {
    const name = this.state.name.trim();
    if (!name) return;
    this.props.dispatch(createDatabase(name));
    this.close();
    this.reset();
  }

  @autobind
  handleCancelClick() {
    this.close();
  }

  @autobind
  handleKeyDown(event) {
    if (event.key === 'Enter' && document.activeElement === this.nameInput) {
      this.handleConfirmClick();
    }
  }

  @autobind
  handleKeyUp(event) {
    // Force value to lowercase.
    event.target.value = event.target.value.toLocaleLowerCase();
    this.setState({ name: event.target.value });
  }

  open() {
    this.dialog && this.dialog.open();
  }

  close() {
    this.dialog && this.dialog.close();
  }

  reset() {
    this.nameInput.value = '';
    this.setState({ name: '' });
  }

  render() {
    return (
      <Modal 
        className="CreateDatabase"
        ref={ref => this.dialog = ref} 
        title="New database"
        width={450}>
        <div className="create-dialog-content layout vertical">
          <input className="NameInput"
            ref={ref => this.nameInput = ref}
            type="text"
            name="name"
            onKeyUp={this.handleKeyUp}
            onKeyDown={this.handleKeyDown}
            placeholder="Database name" />
          <div style={{ height: '20px' }} />
          <div className="layout horizontal-reverse center">
            <Button 
              disabled={!this.state.name}
              color="green"
              solid
              onClick={this.handleConfirmClick}>
              Create
            </Button>
            <Button 
              color="grey"
              solid
              secondary
              onClick={this.handleCancelClick}>
              Cancel
            </Button>
          </div>
        </div>
      </Modal>
    );
  }
}

CreateDatabase.propTypes = {
  dispatch: PropTypes.func.isRequired
};

export default ReactRedux.connect(() => {
  return {};
}, null, null, { withRef: true })(CreateDatabase);