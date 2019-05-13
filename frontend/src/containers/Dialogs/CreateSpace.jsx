import React, { Component } from 'react';
import * as ReactRedux from 'react-redux';
import PropTypes from 'prop-types';
import autobind from 'autobind-decorator';

import SpaceInvite from 'containers/SpaceInvite';
import Button from 'components/Button';
import Input from 'components/Input';
import Modal from 'components/Modal';
import { createSpace } from 'actions/spaces';

import './CreateSpace.scss';

class CreateSpace extends Component {
  constructor(props) {
    super(props);

    this.state = {
      name: ''
    };
  }

  @autobind
  handleInputChange(value) {
    // Remove all alpha numeric chars, except hyphen.
    this.setState({ 
      name: value.toLowerCase().replace(/\s/g, '-').replace(/[^\w-]+/g, '').trim()
    });
  }

  @autobind
  handleConfirmClick() {
    if (this.isValid()) {
      // TODO: Temporary disabled members due to api bug.
      const members = []; //this.spaceInviteInput.getMembers();
      this.props.dispatch(createSpace(this.state.name, members));
      this.close();
    }
  }

  @autobind
  handleCancelClick() {
    this.close();
  }

  open() {
    this.dialog && this.dialog.open();
    this.input.focus();
  }

  close() {
    this.dialog && this.dialog.close();
    this.reset();
  }

  reset() {
    this.setState({  name: '' });
  }

  isValid() {
    return this.state.name.trim() !== '';
  }

  render() {
    return (
      <Modal
        className="CreateSpace"
        ref={ref => this.dialog = ref} 
        title="New Space"
        width={450}>
        <div className="create-dialog-content layout vertical">
          <Input
            ref={ref => this.input = ref}
            onChange={this.handleInputChange}
            placeholder="space-name"
            theme="light"
            value={this.state.name}
          />

          {/* TODO: Temporary disabled members due to api bug. */}
          {/* <h3>Invite others</h3>
          <SpaceInvite
            ref={ref => this.spaceInviteInput = ref} 
          /> */}

          <div className="Buttons layout horizontal-reverse center">
            <Button
              disabled={!this.isValid()}
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

CreateSpace.propTypes = {
  dispatch: PropTypes.func.isRequired
};

export default ReactRedux.connect(() => {
  return {};
}, null, null, { withRef: true })(CreateSpace);